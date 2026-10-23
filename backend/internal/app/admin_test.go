package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/seed"
)

func seedAdmin(t *testing.T, h *harness, ts tenants) *browser {
	t.Helper()
	_, err := seed.Account(context.Background(), h.db.App, ts.kazutb, seed.Member{
		Email: "rector-office@qadam.test", FullName: "Жанар Мұратова", Locale: "ru", Password: password,
		Roles: []string{"tenant_admin"}, Slug: "zhanar-muratova",
	})
	if err != nil {
		t.Fatal(err)
	}
	b := h.browser(t, kazutbHost)
	login(t, b, "rector-office@qadam.test", password)
	return b
}

func TestMemberAdministration(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	student := h.browser(t, kazutbHost)
	session := register(t, student, "yerbol@qadam.test", "Ербол Жақсылықов")

	expectProblem(t, student.do(http.MethodGet, "/api/v1/admin/members", nil), http.StatusForbidden, "rbac.forbidden")
	rec := admin.do(http.MethodGet, "/api/v1/admin/members?limit=1", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/members", rec)
	page := decode[apigen.MemberPage](t, rec)
	if len(page.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("first page: %+v", page)
	}
	rec = admin.do(http.MethodGet, "/api/v1/admin/members?limit=1&cursor="+*page.NextCursor, nil)
	second := decode[apigen.MemberPage](t, rec)
	if len(second.Items) != 1 || second.Items[0].User.Id == page.Items[0].User.Id {
		t.Fatalf("second page: %+v", second)
	}
	// A page size outside the contract is refused, not quietly changed.
	for _, limit := range []string{"0", "101"} {
		p := expectProblem(t, admin.do(http.MethodGet, "/api/v1/admin/members?limit="+limit, nil), http.StatusBadRequest, "request.invalid_limit")
		if p.Params["max"] != float64(100) {
			t.Fatalf("limit=%s: %+v", limit, p)
		}
	}
	rec = admin.do(http.MethodGet, "/api/v1/admin/members?q=%D0%B5%D1%80%D0%B1%D0%BE%D0%BB", nil)
	if items := decode[apigen.MemberPage](t, rec).Items; len(items) != 1 || string(items[0].Email) != "yerbol@qadam.test" {
		t.Fatalf("search: %+v", items)
	}

	// A change of roles invalidates the access token carrying the old ones.
	target := "/api/v1/admin/members/" + session.User.Id.String()
	rec = admin.do(http.MethodPatch, target, apigen.MemberUpdate{Roles: &[]apigen.Role{apigen.RoleStudent, apigen.RoleMentor}})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPatch, target, rec)
	expectProblem(t, student.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expect(t, student.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusNoContent)
	roles := decode[apigen.Session](t, student.do(http.MethodGet, "/api/v1/auth/session", nil)).Membership.Roles
	if len(roles) != 2 || roles[1] != apigen.RoleMentor {
		t.Fatalf("roles after refresh: %v", roles)
	}

	self := "/api/v1/admin/members/" + decode[apigen.Session](t, admin.do(http.MethodGet, "/api/v1/auth/session", nil)).User.Id.String()
	expectProblem(t, admin.do(http.MethodPatch, self, apigen.MemberUpdate{Roles: &[]apigen.Role{apigen.RoleStudent}}), http.StatusConflict, "member.cannot_demote_self")
	expectProblem(t, admin.do(http.MethodPatch, target, apigen.MemberUpdate{Roles: &[]apigen.Role{}}), http.StatusUnprocessableEntity, "validation.failed")

	// Suspension ends the member's sessions at once.
	suspended := apigen.MemberUpdateStatusSuspended
	expect(t, admin.do(http.MethodPatch, target, apigen.MemberUpdate{Status: &suspended}), http.StatusOK)
	expectProblem(t, student.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expectProblem(t, student.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail("yerbol@qadam.test"), Password: password}), http.StatusForbidden, "auth.account_suspended")
}

func TestAdministratorsCannotDemoteEachOtherAtOnce(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	first := seedAdmin(t, h, ts)
	_, err := seed.Account(context.Background(), h.db.App, ts.kazutb, seed.Member{
		Email: "second-admin@qadam.test", FullName: "Екінші Әкімші", Locale: "ru", Password: password,
		Roles: []string{"tenant_admin"}, Slug: "ekinshi-akimshi",
	})
	if err != nil {
		t.Fatal(err)
	}
	second := h.browser(t, kazutbHost)
	login(t, second, "second-admin@qadam.test", password)
	id := func(b *browser) string {
		return decode[apigen.Session](t, b.do(http.MethodGet, "/api/v1/auth/session", nil)).User.Id.String()
	}
	firstID, secondID := id(first), id(second)

	// Hold the administrators' rows so that both requests arrive before either commits.
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	hold, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hold.Exec(ctx, "SELECT 1 FROM memberships WHERE 'tenant_admin' = ANY(roles) FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	demote := apigen.MemberUpdate{Roles: &[]apigen.Role{apigen.RoleStudent}}
	results := make(chan int, 2)
	go func() { results <- first.do(http.MethodPatch, "/api/v1/admin/members/"+secondID, demote).Code }()
	go func() { results <- second.do(http.MethodPatch, "/api/v1/admin/members/"+firstID, demote).Code }()
	waitForLockWaits(t, h, 2)
	if err := hold.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	codes := []int{<-results, <-results}
	slices.Sort(codes)
	if codes[0] != http.StatusOK || codes[1] != http.StatusConflict {
		t.Fatalf("answers %v, want one 200 and one 409", codes)
	}
	var admins int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM memberships WHERE 'tenant_admin' = ANY(roles) AND status = 'active'").Scan(&admins); err != nil {
		t.Fatal(err)
	}
	if admins != 1 {
		t.Fatalf("%d administrators left", admins)
	}
}

func TestMemberChangeIsNotSavedWithoutRevocation(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	student := register(t, h.browser(t, kazutbHost), "kept@qadam.test", "Өзгермейтін Студент")
	actor := decode[apigen.Session](t, admin.do(http.MethodGet, "/api/v1/auth/session", nil))

	// The revocation store is down: the change must not be saved half-way.
	h.redis.Close()
	ctx := tenancy.With(context.Background(), tenancy.Tenant{ID: ts.kazutb, Slug: "kazutb"})
	ctx = db.WithTenant(ctx, ts.kazutb)
	ctx = authz.WithPrincipal(ctx, authz.Principal{UserID: actor.User.Id, TenantID: ts.kazutb, Roles: []authz.Role{authz.RoleTenantAdmin}})
	suspended := apigen.MemberUpdateStatusSuspended
	for _, update := range []apigen.MemberUpdate{
		{Roles: &[]apigen.Role{apigen.RoleStudent, apigen.RoleMentor}},
		{Status: &suspended},
	} {
		if _, err := h.services.Users.UpdateMember(ctx, student.User.Id, update); err == nil {
			t.Fatalf("%+v saved without revoking access", update)
		}
	}
	var roles []string
	var status string
	conn, err := pgx.Connect(context.Background(), h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	err = conn.QueryRow(context.Background(), "SELECT roles, status FROM memberships WHERE user_id = $1 AND tenant_id = $2",
		student.User.Id, ts.kazutb).Scan(&roles, &status)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "student" || status != "active" {
		t.Fatalf("membership changed: %v %s", roles, status)
	}
}

func TestImportCodesIgnoreCaseAndInvitationsAreLimited(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	upload := func(kind, data string) apigen.ImportJob {
		t.Helper()
		rec := admin.do(http.MethodPost, "/api/v1/admin/imports", multipartForm(t, map[string]string{"kind": kind}, "file", kind+".csv", []byte(data)))
		expect(t, rec, http.StatusCreated)
		return decode[apigen.ImportJob](t, rec)
	}
	commit := func(job apigen.ImportJob) *httptest.ResponseRecorder {
		return admin.do(http.MethodPost, "/api/v1/admin/imports/"+job.Id.String()+"/commit", nil)
	}

	// A faculty imported as "ROB" and later as "rob" is one faculty.
	expect(t, commit(upload("structure", "Код факультета;Название факультета;Код кафедры;Название кафедры\n"+
		"ROB;Факультет робототехники;MECH;Кафедра мехатроники\n")), http.StatusOK)
	again := upload("structure", "Факультет коды;Факультет атауы;Кафедра коды;Кафедра атауы\n"+
		"rob;Робототехника факультеті;mech;Мехатроника кафедрасы\n")
	if again.Summary.ToCreate != 0 || again.Summary.ToUpdate != 1 {
		t.Fatalf("second structure preview: %+v", again.Summary)
	}
	expect(t, commit(again), http.StatusOK)
	found := 0
	for _, f := range decode[struct{ Items []apigen.Faculty }](t, admin.do(http.MethodGet, "/api/v1/faculties", nil)).Items {
		if strings.EqualFold(f.Code, "rob") {
			found++
			if f.Name.Ru == nil || f.Name.Kk == nil || len(f.Departments) != 1 {
				t.Fatalf("merged faculty: %+v", f)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d faculties for one code", found)
	}

	// Invitation letters come out of a daily allowance, all or none.
	h.services.Imports.LimitInvitations(ratelimit.Limit{Rate: 2, Burst: 2, Period: 24 * time.Hour})
	students := upload("students", "почта,фио\ninv1@qadam.test,Бірінші Студент\ninv2@qadam.test,Екінші Студент\ninv3@qadam.test,Үшінші Студент\n")
	expectProblem(t, commit(students), http.StatusTooManyRequests, "import.too_many_invitations")
	if items := decode[apigen.MemberPage](t, admin.do(http.MethodGet, "/api/v1/admin/members?q=inv1", nil)).Items; len(items) != 0 {
		t.Fatalf("a refused import was applied: %+v", items)
	}
	h.services.Imports.LimitInvitations(ratelimit.Limit{Rate: 10, Burst: 10, Period: 24 * time.Hour})
	rec := commit(students)
	expect(t, rec, http.StatusOK)
	if res := decode[apigen.ImportResult](t, rec); res.Invited != 3 {
		t.Fatalf("invited %d", res.Invited)
	}
}

func cp1251(t *testing.T, s string) []byte {
	t.Helper()
	b, err := charmap.Windows1251.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func xlsx(t *testing.T, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.SetSheetRow("Sheet1", cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImports(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	upload := func(kind, name string, data []byte) *httptest.ResponseRecorder {
		return admin.do(http.MethodPost, "/api/v1/admin/imports", multipartForm(t, map[string]string{"kind": kind}, "file", name, data))
	}

	// Structure from an Excel-made CSV: Windows-1251, semicolons, Russian headers.
	structure := "Код факультета;Название факультета;Код кафедры;Название кафедры;Группа;Уровень обучения;Год поступления\r\n" +
		"FIT;;IS;;ИС-25-1;бакалавриат;2025\r\n" +
		"FGT;Факультет зелёных технологий;RE;Кафедра возобновляемой энергетики;ВЭ-25-1;Бакалавриат;2025\r\n" +
		"FGT;;RE;;М-ВЭ-25-1;магистратура;\r\n" +
		"NEW;;;;;;\r\n" +
		"FIT;;IS;;ИС-25-2;аспирантура;2025\r\n"
	rec := upload("structure", "structure.csv", cp1251(t, structure))
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, "/api/v1/admin/imports", rec)
	job := decode[apigen.ImportJob](t, rec)
	if job.Summary.Total != 5 || job.Summary.Invalid != 2 || job.Summary.ToCreate != 3 || job.Status != apigen.ImportStatusPreview {
		t.Fatalf("structure preview: %+v", job.Summary)
	}
	if job.Rows[3].Errors[0].Field != "faculty_name_ru" || job.Rows[4].Errors[0].Field != "study_level" || job.Rows[3].Line != 5 {
		t.Fatalf("row errors: %+v / %+v", job.Rows[3], job.Rows[4])
	}
	rec = admin.do(http.MethodPost, "/api/v1/admin/imports/"+job.Id.String()+"/commit", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, "/api/v1/admin/imports/"+job.Id.String()+"/commit", rec)
	if res := decode[apigen.ImportResult](t, rec); res.Created != 3 || res.Skipped != 2 {
		t.Fatalf("structure result: %+v", res)
	}
	expectProblem(t, admin.do(http.MethodPost, "/api/v1/admin/imports/"+job.Id.String()+"/commit", nil), http.StatusConflict, "import.already_committed")
	faculties := decode[struct{ Items []apigen.Faculty }](t, admin.do(http.MethodGet, "/api/v1/faculties", nil)).Items
	found := false
	for _, f := range faculties {
		if f.Code == "FGT" && (f.Name.Ru == nil || *f.Name.Ru != "Факультет зелёных технологий" || f.Name.Kk != nil) {
			t.Fatalf("FGT names: %+v", f.Name)
		}
		found = found || (f.Code == "FGT" && len(f.Departments) == 1)
	}
	if !found {
		t.Fatalf("FGT not created: %+v", faculties)
	}

	expectProblem(t, upload("students", "bad.csv", []byte("email,full_name,salary\na@b.kz,Name,100\n")), http.StatusUnprocessableEntity, "import.unknown_columns")
	expectProblem(t, upload("students", "bad.csv", []byte("full_name\nName\n")), http.StatusUnprocessableEntity, "import.missing_columns")

	// Students from XLSX: valid, unknown group, bad e-mail, duplicate, staff.
	register(t, h.browser(t, kazutbHost), "existing@qadam.test", "Бар Студент")
	rec = upload("students", "students.xlsx", xlsx(t, [][]any{
		{"ФИО", "Почта", "Группа", "Курс", "Роль"},
		{"Әсел Нұрланқызы", "Asel@Qadam.test", "ВЭ-25-1", 1, ""},
		{"Бар Студент", "existing@qadam.test", "ИС-25-1", 1, ""},
		{"Нет Группы", "nogroup@qadam.test", "XX-99", 1, ""},
		{"Плохая Почта", "not-an-email", "ИС-25-1", 1, ""},
		{"Двойник", "asel@qadam.test", "ИС-25-1", 1, ""},
		{"Мұғалім Ұстаз", "teacher@qadam.test", "", "", "научный руководитель, ментор"},
		{"Хитрец", "hacker@qadam.test", "", "", "tenant_admin"},
	}))
	expect(t, rec, http.StatusCreated)
	job = decode[apigen.ImportJob](t, rec)
	if job.Summary.Invalid != 4 || job.Summary.ToCreate != 2 || job.Summary.ToUpdate != 1 {
		t.Fatalf("students preview: %+v %+v", job.Summary, job.Rows)
	}
	want := map[int]string{3: "group_code", 4: "email", 5: "email", 7: "roles"}
	for i, field := range want {
		if job.Rows[i-1].Errors[0].Field != field {
			t.Fatalf("row %d: %+v", i, job.Rows[i-1])
		}
	}
	rec = admin.do(http.MethodGet, "/api/v1/admin/imports/"+job.Id.String(), nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/imports/"+job.Id.String(), rec)
	rec = admin.do(http.MethodPost, "/api/v1/admin/imports/"+job.Id.String()+"/commit", nil)
	expect(t, rec, http.StatusOK)
	if res := decode[apigen.ImportResult](t, rec); res.Created != 2 || res.Invited != 2 || res.Updated != 1 || res.Skipped != 4 {
		t.Fatalf("students result: %+v", res)
	}

	// The invitation sets a password, activates the membership and verifies the address.
	invite := h.lastMail(t, "asel@qadam.test", mail.TemplateInvite)
	if !strings.Contains(invite.Data["link"], "/reset-password?token=") || !strings.HasSuffix(invite.Data["link"], "&invite=1") {
		t.Fatalf("invite link: %s", invite.Data["link"])
	}
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail("asel@qadam.test"), Password: password}), http.StatusUnauthorized, "auth.invalid_credentials")
	asel := h.browser(t, kazutbHost)
	expect(t, asel.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: linkToken(t, invite), Password: password}), http.StatusNoContent)
	session := login(t, asel, "asel@qadam.test", password)
	if session.Membership.Status != apigen.MembershipStatusActive || !session.User.EmailVerified ||
		session.Membership.Group == nil || session.Membership.Group.Code != "ВЭ-25-1" {
		t.Fatalf("invited member: %+v", session)
	}
	members := decode[apigen.MemberPage](t, admin.do(http.MethodGet, "/api/v1/admin/members?role=mentor", nil)).Items
	if len(members) != 1 || members[0].User.FullName != "Мұғалім Ұстаз" || members[0].Status != apigen.MembershipStatusInvited {
		t.Fatalf("staff import: %+v", members)
	}
}
