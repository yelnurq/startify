package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/seed"
)

const (
	kazutbHost  = "kazutb.localhost"
	sandboxHost = "sandbox.localhost"
	password    = "Steppe-Falcon-Rises-42"
)

type tenants struct {
	kazutb  uuid.UUID
	sandbox uuid.UUID
}

func seedTenants(t *testing.T, h *harness) tenants {
	t.Helper()
	ctx := context.Background()
	k, err := seed.Tenant(ctx, h.db.App, seed.KazUTB)
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Structure(ctx, h.db.App, k.ID, seed.KazUTBStructure); err != nil {
		t.Fatal(err)
	}
	s, err := seed.Tenant(ctx, h.db.App, seed.Sandbox)
	if err != nil {
		t.Fatal(err)
	}
	return tenants{kazutb: k.ID, sandbox: s.ID}
}

func registration(email, name string) apigen.RegisterRequest {
	return apigen.RegisterRequest{
		Email: openapiEmail(email), FullName: name, Password: password,
		Locale: apigen.LocaleRu, ConsentVersion: seed.ConsentVersion,
	}
}

// register signs a student up as the web client and the mailbox would: a new
// account opens the verification link from its letter, which signs it in; an
// account with a verified e-mail joins the university with its password.
func register(t *testing.T, b *browser, email, name string) apigen.Session {
	t.Helper()
	rec := b.do(http.MethodPost, "/api/v1/auth/register", registration(email, name))
	if rec.Code == http.StatusCreated {
		return decode[apigen.Session](t, rec)
	}
	expect(t, rec, http.StatusAccepted)
	return verifyEmail(t, b, email)
}

// verifyEmail opens the latest verification link sent to the address.
func verifyEmail(t *testing.T, b *browser, email string) apigen.Session {
	t.Helper()
	letter := b.h.lastMail(t, strings.ToLower(email), mail.TemplateVerifyEmail)
	rec := b.do(http.MethodPost, "/api/v1/auth/email/verify", apigen.TokenRequest{Token: linkToken(t, letter)})
	expect(t, rec, http.StatusOK)
	return decode[apigen.Session](t, rec)
}

func login(t *testing.T, b *browser, email, pass string) apigen.Session {
	t.Helper()
	rec := b.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail(email), Password: pass})
	expect(t, rec, http.StatusOK)
	return decode[apigen.Session](t, rec)
}

func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 160, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTenantResolution(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)

	rec := h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/tenant", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/tenant", rec)
	tenant := decode[apigen.Tenant](t, rec)
	if tenant.Slug != "kazutb" || tenant.Consent.Version != seed.ConsentVersion || *tenant.ShortName.Ru != "КазУТБ" {
		t.Fatalf("unexpected tenant: %+v", tenant)
	}

	expectProblem(t, h.browser(t, "unknown.example.org").do(http.MethodGet, "/api/v1/tenant", nil), http.StatusNotFound, "tenant.not_found")

	// The structure is for members: visitors (registration needs none of it) do not read it.
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/faculties", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/study-groups?q=%D0%B8%D1%81-23", nil),
		http.StatusUnauthorized, "auth.unauthenticated")
	member := h.browser(t, kazutbHost)
	register(t, member, "structure@qadam.test", "Құрылым Оқушы")
	rec = member.do(http.MethodGet, "/api/v1/faculties", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/faculties", rec)
	faculties := decode[struct{ Items []apigen.Faculty }](t, rec)
	if len(faculties.Items) != len(seed.KazUTBStructure) || len(faculties.Items[0].Departments) == 0 {
		t.Fatalf("faculties: %+v", faculties)
	}
	rec = member.do(http.MethodGet, "/api/v1/study-groups?q=%D0%B8%D1%81-23", nil)
	expect(t, rec, http.StatusOK)
	groups := decode[struct{ Items []apigen.StudyGroup }](t, rec)
	if len(groups.Items) != 1 || groups.Items[0].Code != "ИС-23-1" {
		t.Fatalf("group search: %+v", groups)
	}
	// Structure of one university is invisible on another.
	other := h.browser(t, sandboxHost)
	register(t, other, "structure@qadam.test", "Құрылым Оқушы")
	rec = other.do(http.MethodGet, "/api/v1/faculties", nil)
	if items := decode[struct{ Items []apigen.Faculty }](t, rec).Items; len(items) != 0 {
		t.Fatalf("sandbox sees faculties: %+v", items)
	}
}

func TestMemberDirectoryIsForTheUniversitysPeople(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	student := h.browser(t, kazutbHost)
	register(t, student, "directory@qadam.test", "Каталог Студент")
	rec := student.do(http.MethodGet, "/api/v1/people?q=%D0%9A%D0%B0%D1%82", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/people", rec)

	// A partner from outside the university does not list its members.
	partner := h.browser(t, kazutbHost)
	session := register(t, partner, "investor@qadam.test", "Инвестор Серіктес")
	h.db.Exec(t, "UPDATE memberships SET roles = '{partner}' WHERE user_id = '"+session.User.Id.String()+"'")
	login(t, partner, "investor@qadam.test", password)
	expectProblem(t, partner.do(http.MethodGet, "/api/v1/people", nil), http.StatusForbidden, "rbac.forbidden")

	// Users are global, yet only members of the university are described by id.
	outsider := register(t, h.browser(t, sandboxHost), "outside@qadam.test", "Сырттағы Адам")
	ctx := db.WithTenant(context.Background(), ts.kazutb)
	briefs, err := h.services.Users.Briefs(ctx, []uuid.UUID{outsider.User.Id, session.User.Id})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := briefs[outsider.User.Id]; ok || len(briefs) != 1 {
		t.Fatalf("briefs of another university's user: %+v", briefs)
	}
}

func TestRegistrationProfileAndSession(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	b := h.browser(t, kazutbHost)

	weak := registration("aru@qadam.test", "Ару Бекова")
	weak.Password = "qwerty12345"
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/register", weak), http.StatusUnprocessableEntity, "auth.password_weak")
	stale := registration("aru@qadam.test", "Ару Бекова")
	stale.ConsentVersion = "2020-01"
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/register", stale), http.StatusUnprocessableEntity, "auth.consent_required")

	// Registration only sends the verification letter: no session yet.
	rec := b.do(http.MethodPost, "/api/v1/auth/register", registration("Aru@Qadam.test", "Ару Бекова"))
	expect(t, rec, http.StatusAccepted)
	h.validate(t, http.MethodPost, "/api/v1/auth/register", rec)
	if !decode[apigen.VerificationSent](t, rec).VerificationSent || len(b.jar) != 0 {
		t.Fatalf("registration started a session: %s %v", rec.Body, b.jar)
	}
	letter := h.lastMail(t, "aru@qadam.test", mail.TemplateVerifyEmail)
	if !strings.HasPrefix(letter.Data["link"], "http://kazutb.localhost:3000/verify-email?token=") || letter.Locale != "ru" {
		t.Fatalf("verification letter: %+v", letter)
	}

	// The link signs in and confirms the address, once.
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/email/verify", apigen.TokenRequest{Token: "forged"}), http.StatusBadRequest, "auth.token_invalid")
	rec = b.do(http.MethodPost, "/api/v1/auth/email/verify", apigen.TokenRequest{Token: linkToken(t, letter)})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, "/api/v1/auth/email/verify", rec)
	session := decode[apigen.Session](t, rec)
	if session.User.Email != "aru@qadam.test" || !session.User.EmailVerified || session.Membership.Onboarded ||
		len(session.Membership.Roles) != 1 || session.Membership.Roles[0] != apigen.RoleStudent {
		t.Fatalf("unexpected session: %+v", session)
	}
	for _, name := range []string{"qadam_at", "qadam_rt", "qadam_csrf"} {
		c, ok := b.jar[name]
		if !ok {
			t.Fatalf("cookie %s not set", name)
		}
		if name != "qadam_csrf" && !c.HttpOnly {
			t.Fatalf("cookie %s must be httpOnly", name)
		}
	}
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/email/verify", apigen.TokenRequest{Token: linkToken(t, letter)}),
		http.StatusBadRequest, "auth.token_invalid")
	rec = b.do(http.MethodGet, "/api/v1/auth/session", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/auth/session", rec)
	// The address is taken; the right password would prove ownership, another one does not.
	taken := registration("aru@qadam.test", "Ару")
	taken.Password = "Different-Password-9"
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/register", taken), http.StatusConflict, "auth.email_taken")

	// Onboarding fills the faculty and the level from the group.
	rec = b.do(http.MethodGet, "/api/v1/study-groups?q="+url.QueryEscape("ИС-23"), nil)
	group := decode[struct{ Items []apigen.StudyGroup }](t, rec).Items[0]
	course := 2
	rec = b.do(http.MethodPut, "/api/v1/me/onboarding", apigen.OnboardingRequest{
		Locale: apigen.LocaleKk, GroupId: &group.Id, Course: &course, HasIdea: true, Idea: ptr("Кампустағы асхана кезегін болжайтын сервис"),
		Skills: []string{"Python", " python ", "SQL"}, Interests: []string{"EdTech"},
	})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, "/api/v1/me/onboarding", rec)
	me := decode[apigen.Me](t, rec)
	if !me.Membership.Onboarded || me.Membership.Faculty == nil || me.Membership.Faculty.Code != "FIT" ||
		me.Membership.Group.Code != "ИС-23-1" || *me.Membership.StudyLevel != apigen.StudyLevelBachelor ||
		me.User.Locale != apigen.LocaleKk || len(me.Profile.Skills) != 2 || me.Profile.PublicSlug != "aru-bekova" {
		t.Fatalf("onboarding result: %+v", me)
	}
	missing := b.do(http.MethodPut, "/api/v1/me/onboarding", apigen.OnboardingRequest{Locale: apigen.LocaleRu, Skills: []string{}, Interests: []string{}})
	expectProblem(t, missing, http.StatusUnprocessableEntity, "validation.failed")
	if !strings.Contains(missing.Body.String(), `"field":"faculty_id","code":"required"`) {
		t.Fatalf("expected faculty_id required: %s", missing.Body)
	}

	// Profile: a taken address is a field error, links are checked.
	other := h.browser(t, kazutbHost)
	register(t, other, "bolat@qadam.test", "Болат Серіков")
	profile := apigen.ProfileUpdate{
		Headline: ptr("Студент ИС, люблю данные"), Bio: ptr("Собираю команду для проекта про очереди в столовой."),
		Skills: []string{"Python"}, Interests: []string{"EdTech"}, PublicSlug: "bolat-serikov", IsPublic: true, LookingForTeam: true,
		Links: apigen.ProfileLinks{Github: ptr("https://evil.example/aru"), Telegram: ptr("@aru_bekova")},
	}
	rec = b.do(http.MethodPut, "/api/v1/me/profile", profile)
	expectProblem(t, rec, http.StatusUnprocessableEntity, "validation.failed")
	for _, want := range []string{`"field":"links.github","code":"url"`, `"field":"public_slug","code":"taken"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("missing %s in %s", want, rec.Body)
		}
	}
	profile.PublicSlug, profile.Links.Github = "aru", ptr("github.com/aru-bekova")
	rec = b.do(http.MethodPut, "/api/v1/me/profile", profile)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, "/api/v1/me/profile", rec)
	me = decode[apigen.Me](t, rec)
	if *me.Profile.Links.Telegram != "aru_bekova" || *me.Profile.Links.Github != "https://github.com/aru-bekova" {
		t.Fatalf("links not normalised: %+v", me.Profile.Links)
	}

	// The public profile is visible without signing in, but only on its university.
	rec = h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/profiles/aru", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/profiles/aru", rec)
	if p := decode[apigen.PublicProfile](t, rec); p.FullName != "Ару Бекова" || p.Faculty == nil || p.Tenant.Slug != "kazutb" {
		t.Fatalf("public profile: %+v", p)
	}
	expectProblem(t, h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/profiles/aru", nil), http.StatusNotFound, "profile.not_found")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/profiles/bolat-serikov", nil), http.StatusNotFound, "profile.not_found")

	// Avatar: cropped to a square JPEG, served with caching headers.
	expectProblem(t, b.do(http.MethodPut, "/api/v1/me/avatar", multipartForm(t, nil, "file", "notes.txt", []byte("not an image at all"))),
		http.StatusUnprocessableEntity, "files.unsupported_type")
	rec = b.do(http.MethodPut, "/api/v1/me/avatar", multipartForm(t, nil, "file", "photo.png", pngImage(t, 900, 600)))
	expect(t, rec, http.StatusOK)
	me = decode[apigen.Me](t, rec)
	if me.User.AvatarUrl == nil {
		t.Fatal("avatar url missing")
	}
	rec = h.browser(t, kazutbHost).do(http.MethodGet, *me.User.AvatarUrl, nil)
	expect(t, rec, http.StatusOK)
	if rec.Header().Get("Content-Type") != "image/jpeg" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("avatar headers: %v", rec.Header())
	}
	avatar, _, err := image.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	if err != nil || avatar.Width != 512 || avatar.Height != 512 {
		t.Fatalf("avatar is %dx%d (%v)", avatar.Width, avatar.Height, err)
	}
	etag := rec.Header().Get("ETag")
	rec = h.browser(t, kazutbHost).do(http.MethodGet, *me.User.AvatarUrl, nil, func(r *http.Request) { r.Header.Set("If-None-Match", etag) })
	expect(t, rec, http.StatusNotModified)
	expect(t, b.do(http.MethodDelete, "/api/v1/me/avatar", nil), http.StatusNoContent)
	if me := decode[apigen.Me](t, b.do(http.MethodGet, "/api/v1/me", nil)); me.User.AvatarUrl != nil {
		t.Fatal("avatar not removed")
	}

	// Sessions: list, sign out, sign in again.
	rec = b.do(http.MethodGet, "/api/v1/auth/sessions", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/auth/sessions", rec)
	sessions := decode[struct{ Items []apigen.SessionInfo }](t, rec).Items
	if len(sessions) != 1 || !sessions[0].Current || *sessions[0].UserAgent != "qadam-test" {
		t.Fatalf("sessions: %+v", sessions)
	}
	expect(t, b.do(http.MethodPost, "/api/v1/auth/logout", nil), http.StatusNoContent)
	if len(b.jar) != 0 {
		t.Fatalf("cookies left after logout: %v", b.jar)
	}
	expectProblem(t, b.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail("aru@qadam.test"), Password: "wrong-password-1"}),
		http.StatusUnauthorized, "auth.invalid_credentials")
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail("nobody@qadam.test"), Password: password}),
		http.StatusUnauthorized, "auth.invalid_credentials")
	if s := login(t, b, "aru@qadam.test", password); s.User.FullName != "Ару Бекова" {
		t.Fatalf("login: %+v", s)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	b := h.browser(t, kazutbHost)
	register(t, b, "dias@qadam.test", "Диас Омаров")
	first := b.jar["qadam_rt"].Value
	oldAccess := b.jar["qadam_at"].Value

	expect(t, b.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusNoContent)
	second := b.jar["qadam_rt"].Value
	if second == first || b.jar["qadam_at"].Value == oldAccess {
		t.Fatal("tokens were not rotated")
	}

	// A parallel tab presenting the previous token within the grace period
	// gets 204 without new cookies.
	stale := h.browser(t, kazutbHost)
	stale.jar["qadam_rt"] = &http.Cookie{Name: "qadam_rt", Value: first, Path: "/api/v1/auth"}
	stale.jar["qadam_csrf"] = b.jar["qadam_csrf"]
	rec := stale.do(http.MethodPost, "/api/v1/auth/refresh", nil)
	expect(t, rec, http.StatusNoContent)
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("grace refresh must not set cookies")
	}

	// After the grace period the old token is a stolen copy: the session ends.
	h.db.Exec(t, "UPDATE sessions SET rotated_at = now() - interval '5 minutes'")
	stale.jar["qadam_rt"] = &http.Cookie{Name: "qadam_rt", Value: first, Path: "/api/v1/auth"}
	expectProblem(t, stale.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	if _, ok := stale.jar["qadam_rt"]; ok {
		t.Fatal("cookies must be cleared")
	}
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	// The access token of the revoked session is rejected before it expires.
	held := h.browser(t, kazutbHost)
	held.jar["qadam_at"] = &http.Cookie{Name: "qadam_at", Value: oldAccess, Path: "/api"}
	expectProblem(t, held.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
}

func TestLoginCodeAndPasswordRecovery(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	register(t, h.browser(t, kazutbHost), "saule@qadam.test", "Сәуле Әлиева")

	b := h.browser(t, kazutbHost)
	// Unknown addresses get the same answer and no letter.
	expect(t, b.do(http.MethodPost, "/api/v1/auth/otp/request", apigen.EmailRequest{Email: openapiEmail("ghost@qadam.test")}), http.StatusAccepted)
	expect(t, b.do(http.MethodPost, "/api/v1/auth/otp/request", apigen.EmailRequest{Email: openapiEmail("saule@qadam.test")}), http.StatusAccepted)
	for _, m := range h.mails(t) {
		if m.To == "ghost@qadam.test" {
			t.Fatal("letter sent to an unknown address")
		}
	}
	code := h.lastMail(t, "saule@qadam.test", mail.TemplateLoginCode).Data["code"]
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for range 5 {
		expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/otp/verify", apigen.LoginCodeRequest{Email: openapiEmail("saule@qadam.test"), Code: wrong}),
			http.StatusUnauthorized, "auth.code_invalid")
	}
	// Five wrong attempts burn the code.
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/otp/verify", apigen.LoginCodeRequest{Email: openapiEmail("saule@qadam.test"), Code: code}),
		http.StatusUnauthorized, "auth.code_invalid")
	expect(t, b.do(http.MethodPost, "/api/v1/auth/otp/request", apigen.EmailRequest{Email: openapiEmail("saule@qadam.test")}), http.StatusAccepted)
	code = h.lastMail(t, "saule@qadam.test", mail.TemplateLoginCode).Data["code"]
	rec := b.do(http.MethodPost, "/api/v1/auth/otp/verify", apigen.LoginCodeRequest{Email: openapiEmail("saule@qadam.test"), Code: code})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, "/api/v1/auth/otp/verify", rec)
	if !decode[apigen.Session](t, rec).User.EmailVerified {
		t.Fatal("a code proves the address")
	}

	// Password reset ends every session.
	expect(t, b.do(http.MethodPost, "/api/v1/auth/password/forgot", apigen.EmailRequest{Email: openapiEmail("saule@qadam.test")}), http.StatusAccepted)
	token := linkToken(t, h.lastMail(t, "saule@qadam.test", mail.TemplatePasswordReset))
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: token, Password: "short"}),
		http.StatusUnprocessableEntity, "auth.password_weak")
	expect(t, b.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: token, Password: "New-Steppe-Password-7"}), http.StatusNoContent)
	expectProblem(t, b.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: token, Password: "Another-Password-77"}),
		http.StatusBadRequest, "auth.token_invalid")
	login(t, b, "saule@qadam.test", "New-Steppe-Password-7")

	// Changing the password needs the current one and keeps this session.
	expectProblem(t, b.do(http.MethodPut, "/api/v1/auth/password", apigen.PasswordChangeRequest{CurrentPassword: "nope-nope-nope", NewPassword: "Third-Password-777"}),
		http.StatusUnprocessableEntity, "auth.password_incorrect")
	expect(t, b.do(http.MethodPut, "/api/v1/auth/password", apigen.PasswordChangeRequest{CurrentPassword: "New-Steppe-Password-7", NewPassword: "Third-Password-777"}),
		http.StatusNoContent)
	expect(t, b.do(http.MethodGet, "/api/v1/me", nil), http.StatusOK)
}

func TestCSRFProtection(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	b := h.browser(t, kazutbHost)
	register(t, b, "erzhan@qadam.test", "Ержан Төлеуов")
	body := apigen.MeUpdate{FullName: ptr("Ержан Төлеуұлы")}

	noHeader := func(r *http.Request) { r.Header.Del("X-CSRF-Token") }
	expectProblem(t, b.do(http.MethodPatch, "/api/v1/me", body, noHeader), http.StatusForbidden, "auth.csrf_failed")
	crossSite := func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }
	expectProblem(t, b.do(http.MethodPatch, "/api/v1/me", body, crossSite), http.StatusForbidden, "auth.csrf_failed")
	sibling := func(r *http.Request) { r.Header.Set("Origin", "http://sandbox.localhost") }
	expectProblem(t, b.do(http.MethodPatch, "/api/v1/me", body, sibling), http.StatusForbidden, "auth.csrf_failed")
	rec := b.do(http.MethodPatch, "/api/v1/me", body)
	expect(t, rec, http.StatusOK)
	if decode[apigen.Me](t, rec).User.FullName != "Ержан Төлеуұлы" {
		t.Fatal("name not updated")
	}
	// Signing in again with stale cookies but no CSRF cookie still works.
	delete(b.jar, "qadam_csrf")
	login(t, b, "erzhan@qadam.test", password)
}

func TestTenantIsolationOfSessions(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	k := h.browser(t, kazutbHost)
	register(t, k, "alina@qadam.test", "Алина Ким")

	// Cookies are per host, but even a copied access token is rejected elsewhere.
	s := h.browser(t, sandboxHost)
	s.jar["qadam_at"] = k.jar["qadam_at"]
	expectProblem(t, s.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	s.jar = map[string]*http.Cookie{"qadam_rt": k.jar["qadam_rt"], "qadam_csrf": k.jar["qadam_csrf"]}
	expectProblem(t, s.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_expired")

	// The account exists, but is not a member of the sandbox university...
	s = h.browser(t, sandboxHost)
	expectProblem(t, s.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail("alina@qadam.test"), Password: password}),
		http.StatusForbidden, "auth.not_a_member")
	// ...until it joins with the same password.
	joined := register(t, s, "alina@qadam.test", "Алина Ким")
	if joined.Tenant.Slug != "sandbox" {
		t.Fatalf("joined %+v", joined.Tenant)
	}
	expectProblem(t, s.do(http.MethodPost, "/api/v1/auth/register", func() apigen.RegisterRequest {
		r := registration("alina@qadam.test", "Алина Ким")
		r.Password = "Not-The-Right-One-1"
		return r
	}()), http.StatusConflict, "auth.email_taken")
}

func countAudit(t *testing.T, h *harness, action, userEmail string) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var n int
	err = conn.QueryRow(ctx, `SELECT count(*) FROM audit_log a JOIN users u ON u.id = a.actor_id
		WHERE a.action = $1 AND u.email = $2`, action, userEmail).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNoSessionBeforeTheEmailIsVerified(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	const victim = "rector@qadam.test"
	letters := func() int {
		n := 0
		for _, m := range h.mails(t) {
			if m.To == victim && m.Template == mail.TemplateVerifyEmail {
				n++
			}
		}
		return n
	}

	// Someone registers an address they do not control: no session, and the
	// right password does not sign in until the address is verified.
	squatter := h.browser(t, kazutbHost)
	claim := registration(victim, "Ректор Самозванец")
	claim.Password = "Squatter-Password-13"
	expect(t, squatter.do(http.MethodPost, "/api/v1/auth/register", claim), http.StatusAccepted)
	squatterLink := linkToken(t, h.lastMail(t, victim, mail.TemplateVerifyEmail))
	rec := squatter.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail(victim), Password: claim.Password})
	expectProblem(t, rec, http.StatusForbidden, "auth.email_unverified")
	if len(squatter.jar) != 0 {
		t.Fatalf("cookies after an unverified sign-in: %v", squatter.jar)
	}
	expectProblem(t, squatter.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail(victim), Password: "Not-The-Password-1"}),
		http.StatusUnauthorized, "auth.invalid_credentials")
	expectProblem(t, squatter.do(http.MethodGet, "/api/v1/auth/session", nil), http.StatusUnauthorized, "auth.unauthenticated")

	// The letter can be sent again without a session; unknown addresses get the same answer.
	before := letters()
	rec = h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/email/resend", apigen.EmailRequest{Email: openapiEmail(victim)})
	expect(t, rec, http.StatusAccepted)
	h.validate(t, http.MethodPost, "/api/v1/auth/email/resend", rec)
	expect(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/email/resend", apigen.EmailRequest{Email: openapiEmail("ghost@qadam.test")}),
		http.StatusAccepted)
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/email/resend", nil), http.StatusUnprocessableEntity, "validation.failed")
	if letters() != before+1 {
		t.Fatal("the letter was not sent again")
	}
	for _, m := range h.mails(t) {
		if m.To == "ghost@qadam.test" {
			t.Fatal("letter sent to an unknown address")
		}
	}
	// Each new letter replaces the previous links.
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/email/verify", apigen.TokenRequest{Token: squatterLink}),
		http.StatusBadRequest, "auth.token_invalid")

	// The owner of the mailbox registers too: the unverified account is
	// theirs now, with their name and password; the squatter's password and
	// sessions are gone.
	h.db.Exec(t, "UPDATE users SET email_verified_at = now() WHERE email = '"+victim+"'")
	legacy := h.browser(t, kazutbHost)
	login(t, legacy, victim, claim.Password)
	h.db.Exec(t, "UPDATE users SET email_verified_at = NULL WHERE email = '"+victim+"'")
	owner := h.browser(t, kazutbHost)
	expect(t, owner.do(http.MethodPost, "/api/v1/auth/register", registration(victim, "Ректор Настоящий")), http.StatusAccepted)
	expectProblem(t, legacy.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expectProblem(t, legacy.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	session := verifyEmail(t, owner, victim)
	if session.User.FullName != "Ректор Настоящий" || !session.User.EmailVerified {
		t.Fatalf("claimed account: %+v", session.User)
	}
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail(victim), Password: claim.Password}), http.StatusUnauthorized, "auth.invalid_credentials")
	login(t, h.browser(t, kazutbHost), victim, password)

	// A verified account is no longer claimed by registering: a wrong
	// password is a failed sign-in, counted with those of /auth/login.
	before = countAudit(t, h, "auth.login_failed", victim)
	guesser := h.browser(t, kazutbHost)
	for range 10 {
		expectProblem(t, guesser.do(http.MethodPost, "/api/v1/auth/register", claim), http.StatusConflict, "auth.email_taken")
	}
	if got := countAudit(t, h, "auth.login_failed", victim) - before; got != 10 {
		t.Fatalf("%d failed attempts recorded, want 10", got)
	}
	expectProblem(t, guesser.do(http.MethodPost, "/api/v1/auth/register", claim), http.StatusTooManyRequests, "auth.too_many_attempts")
	expectProblem(t, guesser.do(http.MethodPost, "/api/v1/auth/login", apigen.LoginRequest{Email: openapiEmail(victim), Password: password}),
		http.StatusTooManyRequests, "auth.too_many_attempts")
}

func TestSignInCodeGuessesAreCappedPerAccount(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	const email = "guessed@qadam.test"
	register(t, h.browser(t, kazutbHost), email, "Код Иесі")
	attacker := h.browser(t, kazutbHost)
	guess := func(code string) *httptest.ResponseRecorder {
		return attacker.do(http.MethodPost, "/api/v1/auth/otp/verify", apigen.LoginCodeRequest{Email: openapiEmail(email), Code: code})
	}
	other := func(code string) string {
		if code == "000000" {
			return "111111"
		}
		return "000000"
	}
	newCode := func() string {
		expect(t, attacker.do(http.MethodPost, "/api/v1/auth/otp/request", apigen.EmailRequest{Email: openapiEmail(email)}), http.StatusAccepted)
		return h.lastMail(t, email, mail.TemplateLoginCode).Data["code"]
	}

	// Ten wrong guesses spread over two letters (a code burns after five);
	// each one is recorded.
	for range 2 {
		code := newCode()
		for range 5 {
			expectProblem(t, guess(other(code)), http.StatusUnauthorized, "auth.code_invalid")
		}
	}
	if n := countAudit(t, h, "auth.login_code_failed", email); n != 10 {
		t.Fatalf("%d failed codes recorded, want 10", n)
	}
	// Even the right code of a new letter waits: the allowance is per account.
	expectProblem(t, guess(newCode()), http.StatusTooManyRequests, "auth.too_many_attempts")
	// The password still works.
	login(t, h.browser(t, kazutbHost), email, password)
}

func TestAnswersAboutAddressesTakeTheSameTime(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	register(t, h.browser(t, kazutbHost), "known@qadam.test", "Белгілі Адам")
	b := h.browser(t, kazutbHost)
	for _, path := range []string{"/api/v1/auth/otp/request", "/api/v1/auth/password/forgot", "/api/v1/auth/email/resend"} {
		for _, email := range []string{"ghost@qadam.test", "known@qadam.test"} {
			start := time.Now()
			expect(t, b.do(http.MethodPost, path, apigen.EmailRequest{Email: openapiEmail(email)}), http.StatusAccepted)
			// Unknown addresses used to be answered at once.
			if took := time.Since(start); took < 295*time.Millisecond {
				t.Errorf("%s answered %s in %s", path, email, took)
			}
		}
	}
}

func TestCredentialChangesEndSessionsInEveryUniversity(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	const email = "two-universities@qadam.test"
	k := h.browser(t, kazutbHost)
	register(t, k, email, "Жюри Екі Университет")
	// The same account joins the sandbox university with its password.
	s := h.browser(t, sandboxHost)
	register(t, s, email, "Жюри Екі Университет")
	expect(t, s.do(http.MethodGet, "/api/v1/me", nil), http.StatusOK)
	ended := func(b *browser) {
		t.Helper()
		expectProblem(t, b.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
		expectProblem(t, b.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	}

	// "Sign out everywhere" in one university ends the sessions in the other.
	expect(t, k.do(http.MethodDelete, "/api/v1/auth/sessions", nil), http.StatusNoContent)
	ended(s)
	expect(t, k.do(http.MethodGet, "/api/v1/me", nil), http.StatusOK)

	// So does a change of the password...
	s = h.browser(t, sandboxHost)
	login(t, s, email, password)
	expect(t, k.do(http.MethodPut, "/api/v1/auth/password",
		apigen.PasswordChangeRequest{CurrentPassword: password, NewPassword: "Second-Steppe-Password-8"}), http.StatusNoContent)
	ended(s)
	expect(t, k.do(http.MethodGet, "/api/v1/me", nil), http.StatusOK)

	// ...and a reset from a letter, which ends the current university's sessions too.
	s = h.browser(t, sandboxHost)
	login(t, s, email, "Second-Steppe-Password-8")
	other := h.browser(t, kazutbHost)
	expect(t, other.do(http.MethodPost, "/api/v1/auth/password/forgot", apigen.EmailRequest{Email: openapiEmail(email)}), http.StatusAccepted)
	token := linkToken(t, h.lastMail(t, email, mail.TemplatePasswordReset))
	expect(t, other.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: token, Password: "Third-Steppe-Password-9"}), http.StatusNoContent)
	ended(s)
	ended(k)
	login(t, h.browser(t, sandboxHost), email, "Third-Steppe-Password-9")
}

// waitForLockWaits polls until n queries of the test database wait for a row lock.
func waitForLockWaits(t *testing.T, h *harness, n int) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var waiting int
		err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%d queries never waited for a lock", n)
}

func TestPasswordResetLinkWorksOnceUnderConcurrency(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	const email = "once@qadam.test"
	register(t, h.browser(t, kazutbHost), email, "Бір Рет")
	expect(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/password/forgot", apigen.EmailRequest{Email: openapiEmail(email)}),
		http.StatusAccepted)
	token := linkToken(t, h.lastMail(t, email, mail.TemplatePasswordReset))

	// Another request is using the same link and has not committed yet.
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	first, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(ctx, "UPDATE auth_tokens SET consumed_at = now() WHERE purpose = 'reset_password' AND consumed_at IS NULL"); err != nil {
		t.Fatal(err)
	}
	second := h.browser(t, kazutbHost)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- second.do(http.MethodPost, "/api/v1/auth/password/reset", apigen.PasswordResetRequest{Token: token, Password: "Parallel-Password-11"})
	}()
	waitForLockWaits(t, h, 1)
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	expectProblem(t, <-done, http.StatusBadRequest, "auth.token_invalid")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail(email), Password: "Parallel-Password-11"}), http.StatusUnauthorized, "auth.invalid_credentials")
}

func ptr[T any](v T) *T { return &v }

func openapiEmail(s string) openapi_types.Email { return openapi_types.Email(s) }

func TestPasswordGuessingIsLimited(t *testing.T) {
	h := newHarness(t)
	seedTenants(t, h)
	register(t, h.browser(t, kazutbHost), "target@qadam.test", "Нысана Студент")
	attacker := h.browser(t, kazutbHost)
	guess := apigen.LoginRequest{Email: openapiEmail("target@qadam.test"), Password: "guess-password-01"}
	for range 10 {
		expectProblem(t, attacker.do(http.MethodPost, "/api/v1/auth/login", guess), http.StatusUnauthorized, "auth.invalid_credentials")
	}
	rec := attacker.do(http.MethodPost, "/api/v1/auth/login", guess)
	p := expectProblem(t, rec, http.StatusTooManyRequests, "auth.too_many_attempts")
	if rec.Header().Get("Retry-After") == "" || p.Params["retry_after_seconds"] == nil {
		t.Fatalf("no retry hint: %v %v", rec.Header(), p.Params)
	}
	// Even the right password waits: the limit is per account.
	expectProblem(t, attacker.do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail("target@qadam.test"), Password: password}), http.StatusTooManyRequests, "auth.too_many_attempts")
	// Other accounts behind the same address are not affected.
	register(t, h.browser(t, kazutbHost), "neighbour@qadam.test", "Көрші Студент")
	login(t, attacker, "neighbour@qadam.test", password)
}
