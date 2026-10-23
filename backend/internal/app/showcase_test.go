package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
)

// showcaseProject creates a project with the answers a page is made from.
func showcaseProject(t *testing.T, h *harness, owner *browser, title string) (apigen.Project, string) {
	t.Helper()
	p := newProject(t, h, owner, title)
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{
		"statement": "Фермеры Северного Казахстана теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно.",
		"cost":      "Хозяйство на 300 гектаров теряет около 12 миллионов тенге за сезон.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	expect(t, owner.do(http.MethodPatch, base+"/steps/solution", apigen.StepUpdate{Values: map[string]any{
		"solution": "Мобильное приложение распознаёт болезни по фото листа за 10 секунд, даже без интернета в поле.",
		"uvp":      "Диагноз за 10 секунд без агронома и связи.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	return p, base
}

func pageUpdate(slug string, visibility apigen.ShowcaseVisibility) apigen.ShowcaseEntryUpdate {
	problem := "Фермеры теряют до 20% урожая пшеницы, потому что поздно замечают болезни."
	solution := "Приложение распознаёт болезни по фото листа за 10 секунд, даже без интернета."
	return apigen.ShowcaseEntryUpdate{
		Slug: slug, Title: "AgroScan", Tagline: "Болезни пшеницы по фото за 10 секунд",
		Sections: apigen.ShowcaseSections{Problem: &problem, Solution: &solution},
		Needs:    []apigen.ShowcaseNeed{apigen.ShowcaseNeedInvestment, apigen.ShowcaseNeedPilot},
		Tags:     []string{"AI", "Agro", "ai"}, Visibility: visibility, ShowScore: true, ContactEnabled: true,
	}
}

func coverPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 960, 600))
	for y := range 600 {
		for x := range 960 {
			img.Set(x, y, color.RGBA{R: uint8(x / 4), G: 160, B: uint8(y / 3), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestShowcasePublishing(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	_, memberID := account(t, h, ts, "member@qadam.test", "Айгерим Сапарова", "student")
	student, _ := account(t, h, ts, "reader@qadam.test", "Дана Ермекова", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	anonymous := h.browser(t, kazutbHost)
	p, base := showcaseProject(t, h, owner, "AgroScan")
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: memberID, Role: apigen.ProjectMemberAddRoleMember}), http.StatusCreated)
	expect(t, owner.do(http.MethodPost, base+"/evaluations", nil), http.StatusAccepted)
	runEvaluation(t, h)
	path := base + "/showcase"

	// Before the first save the page is a suggestion from the incubator.
	rec := owner.do(http.MethodGet, path, nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, path, rec)
	e := decode[apigen.ShowcaseEntry](t, rec)
	if e.Saved || e.Slug != "agroscan" || e.Status != apigen.ShowcaseStatusDraft || rec.Header().Get("ETag") != `"0"` ||
		e.Sections.Problem == nil || !strings.Contains(*e.Sections.Problem, "12 миллионов") || e.Tagline != "Диагноз за 10 секунд без агронома и связи." ||
		len(e.Team) != 2 || e.Team[0].Name != "Нұрлан Ахметов" || e.Stage != apigen.ShowcaseStageIdea || e.Url != nil ||
		e.Score == nil || e.Level == nil {
		t.Fatalf("suggestion %+v", e)
	}
	expectProblem(t, owner.do(http.MethodPost, path+"/publish", nil), http.StatusUnprocessableEntity, "showcase.incomplete")

	// Saving needs the version; the address and the video are checked.
	expectProblem(t, owner.do(http.MethodPut, path, pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity)), http.StatusPreconditionRequired, "precondition.required")
	bad := pageUpdate("Agro Scan!", apigen.ShowcaseVisibilityUniversity)
	bad.VideoUrl = ptr("https://example.com/pitch.mp4")
	problem := expectProblem(t, owner.do(http.MethodPut, path, bad, ifMatchHeader(`"0"`)), http.StatusUnprocessableEntity, "validation.failed")
	if len(problem.Errors) != 2 || problem.Errors[0].Field != "slug" || problem.Errors[1].Field != "video_url" {
		t.Fatalf("errors %+v", problem.Errors)
	}
	expectProblem(t, student.do(http.MethodPut, path, pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity), ifMatchHeader(`"0"`)), http.StatusNotFound, "project.not_found")
	update := pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity)
	update.VideoUrl = ptr("https://youtu.be/dQw4w9WgXcQ")
	rec = owner.do(http.MethodPut, path, update, ifMatchHeader(`"0"`))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, path, rec)
	e = decode[apigen.ShowcaseEntry](t, rec)
	if !e.Saved || rec.Header().Get("ETag") != `"1"` || strings.Join(e.Tags, ",") != "ai,agro" || len(e.Needs) != 2 || e.Status != apigen.ShowcaseStatusDraft {
		t.Fatalf("saved %+v", e)
	}
	expectProblem(t, owner.do(http.MethodPut, path, update, ifMatchHeader(`"0"`)), http.StatusPreconditionFailed, "precondition.failed")

	// The cover is cropped to 16:9; replacing it is a change of the page.
	rec = owner.do(http.MethodPut, path+"/cover", multipartForm(t, nil, "file", "cover.png", coverPNG(t)))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, path+"/cover", rec)
	if e = decode[apigen.ShowcaseEntry](t, rec); e.CoverUrl == nil {
		t.Fatalf("cover %+v", e)
	}
	cover := h.browser(t, sandboxHost).do(http.MethodGet, *e.CoverUrl, nil)
	expect(t, cover, http.StatusOK)
	if img, _, err := image.Decode(bytes.NewReader(cover.Body.Bytes())); err != nil || img.Bounds().Dx() != 960 || img.Bounds().Dy() != 540 {
		t.Fatalf("cover image %v %v", img.Bounds(), err)
	}
	expectProblem(t, owner.do(http.MethodPut, path+"/cover", multipartForm(t, nil, "file", "cover.png", []byte("not an image"))),
		http.StatusUnprocessableEntity, "files.unsupported_type")

	// A university page is live at once, for members only.
	rec = owner.do(http.MethodPost, path+"/publish", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, path+"/publish", rec)
	if e = decode[apigen.ShowcaseEntry](t, rec); e.Status != apigen.ShowcaseStatusPublished || e.Url == nil || e.PublishedAt == nil {
		t.Fatalf("published %+v", e)
	}
	if card := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); card.Visibility != apigen.ProjectVisibilityUniversity {
		t.Fatalf("project visibility %s", card.Visibility)
	}
	if n := len(decode[apigen.ShowcasePage](t, anonymous.do(http.MethodGet, "/api/v1/showcase", nil)).Items); n != 0 {
		t.Fatalf("anonymous sees %d university pages", n)
	}
	rec = student.do(http.MethodGet, "/api/v1/showcase", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/showcase", rec)
	if items := decode[apigen.ShowcasePage](t, rec).Items; len(items) != 1 || items[0].Score == nil || items[0].University.Slug != "kazutb" {
		t.Fatalf("member sees %+v", items)
	}
	expectProblem(t, anonymous.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusNotFound, "showcase.not_found")
	expect(t, student.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusOK)

	// Going public needs review; the page stays live inside the university.
	rec = owner.do(http.MethodPut, path, pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(`"2"`))
	expect(t, rec, http.StatusOK)
	if e = decode[apigen.ShowcaseEntry](t, rec); e.Status != apigen.ShowcaseStatusInReview || e.SubmittedAt == nil || e.VideoUrl != nil {
		t.Fatalf("in review %+v", e)
	}
	expectProblem(t, anonymous.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusNotFound, "showcase.not_found")
	expect(t, student.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusOK)
	// The landing counter counts what the visitor can open.
	for who, want := range map[*browser]int{anonymous: 0, student: 1} {
		if n := decode[apigen.ShowcaseHighlights](t, who.do(http.MethodGet, "/api/v1/showcase/highlights", nil)).Counters.Published; n != want {
			t.Fatalf("published counter %d, want %d", n, want)
		}
	}
	expectProblem(t, student.do(http.MethodGet, "/api/v1/admin/showcase", nil), http.StatusForbidden, "rbac.forbidden")
	rec = manager.do(http.MethodGet, "/api/v1/admin/showcase", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/showcase", rec)
	if items := decode[apigen.ShowcaseReviewList](t, rec).Items; len(items) != 1 || items[0].ProjectId != p.Id {
		t.Fatalf("review queue %+v", items)
	}
	review := "/api/v1/admin/showcase/" + p.Id.String() + "/review"
	expectProblem(t, manager.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: false}), http.StatusUnprocessableEntity, "validation.failed")
	rec = manager.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: false, Comment: ptr("Уберите фамилии фермеров из раздела о проблеме.")})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, review, rec)
	if e = decode[apigen.ShowcaseEntry](t, rec); e.Status != apigen.ShowcaseStatusRejected || e.ReviewComment == nil {
		t.Fatalf("rejected %+v", e)
	}
	expectProblem(t, manager.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(e))), http.StatusConflict, "showcase.not_in_review")
	expect(t, student.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusOK)
	if e = decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPost, path+"/publish", nil)); e.Status != apigen.ShowcaseStatusInReview {
		t.Fatalf("resubmitted %+v", e)
	}
	if e = decode[apigen.ShowcaseEntry](t, manager.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(e)))); e.Status != apigen.ShowcaseStatusPublished {
		t.Fatalf("approved %+v", e)
	}
	if card := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); card.Visibility != apigen.ProjectVisibilityPublic {
		t.Fatalf("project visibility %s", card.Visibility)
	}

	// The public catalog: filters, search, the national showcase, facets.
	rec = anonymous.do(http.MethodGet, "/api/v1/showcase", nil)
	page := decode[apigen.ShowcasePage](t, rec)
	if len(page.Items) != 1 || page.Items[0].Url != "http://kazutb.localhost:3000/showcase/agroscan" || page.Items[0].Year != time.Now().Year() ||
		page.Items[0].Score == nil || page.Items[0].CoverUrl == nil {
		t.Fatalf("public catalog %+v", page.Items)
	}
	for query, want := range map[string]int{
		"?q=" + "%D0%BF%D1%88%D0%B5%D0%BD%D0%B8%D1%86": 1, "?q=agro": 1, "?q=%D0%BD%D0%B5%D1%84%D1%82%D1%8C": 0,
		"?industry=agritech": 1, "?industry=fintech": 0, "?stage=idea": 1, "?stage=launch": 0, "?need=pilot": 1, "?need=team": 0,
		"?tag=AI": 1, "?tag=web": 0, "?year=" + strconv.Itoa(time.Now().Year()): 1, "?year=2001": 0,
		"?q=%27%20%7C%7C%20%21": 1,
	} {
		if n := len(decode[apigen.ShowcasePage](t, anonymous.do(http.MethodGet, "/api/v1/showcase"+query, nil)).Items); n != want {
			t.Fatalf("%s: %d results, want %d", query, n, want)
		}
	}
	outsider := h.browser(t, sandboxHost)
	if n := len(decode[apigen.ShowcasePage](t, outsider.do(http.MethodGet, "/api/v1/showcase", nil)).Items); n != 0 {
		t.Fatalf("sandbox catalog shows %d KazUTB pages", n)
	}
	national := decode[apigen.ShowcasePage](t, outsider.do(http.MethodGet, "/api/v1/showcase?scope=national", nil))
	if len(national.Items) != 1 || national.Items[0].University.Slug != "kazutb" {
		t.Fatalf("national %+v", national.Items)
	}
	rec = anonymous.do(http.MethodGet, "/api/v1/showcase/facets", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/showcase/facets", rec)
	facets := decode[apigen.ShowcaseFacets](t, rec)
	if facets.Total != 1 || len(facets.Industries) != 1 || facets.Industries[0].Key != "agritech" || len(facets.Needs) != 2 ||
		len(facets.Tags) != 2 || facets.Needs[0].Key != "investment" {
		t.Fatalf("facets %+v", facets)
	}
	rec = anonymous.do(http.MethodGet, "/api/v1/showcase/highlights", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/showcase/highlights", rec)
	if hl := decode[apigen.ShowcaseHighlights](t, rec); hl.Counters.Projects != 1 || hl.Counters.Published != 1 || hl.Counters.Students != 3 ||
		len(hl.Featured) != 1 {
		t.Fatalf("highlights %+v", hl)
	}
	rec = anonymous.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/showcase/entries/agroscan", rec)
	if pp := decode[apigen.ShowcaseProject](t, rec); pp.Card.Title != "AgroScan" || len(pp.Team) != 2 ||
		pp.ImageUrl != "http://kazutb.localhost:3000/api/v1/showcase/entries/agroscan/og.png?v=3" {
		t.Fatalf("public page %+v", pp)
	}

	// Another university cannot reach the page through the project.
	stranger := h.browser(t, sandboxHost)
	register(t, stranger, "outsider@qadam.test", "Outsider Person")
	expectProblem(t, stranger.do(http.MethodGet, path, nil), http.StatusNotFound, "project.not_found")

	// Taken addresses are refused; unpublishing hides the page.
	_, otherBase := showcaseProject(t, h, student, "AgroScan 2")
	problem = expectProblem(t, student.do(http.MethodPut, otherBase+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity), ifMatchHeader(`"0"`)),
		http.StatusUnprocessableEntity, "validation.failed")
	if problem.Errors[0].Field != "slug" || problem.Errors[0].Code != "taken" {
		t.Fatalf("taken %+v", problem.Errors)
	}
	expect(t, owner.do(http.MethodDelete, path+"/cover", nil), http.StatusOK)
	rec = owner.do(http.MethodPost, path+"/unpublish", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, path+"/unpublish", rec)
	if e = decode[apigen.ShowcaseEntry](t, rec); e.Status != apigen.ShowcaseStatusDraft || e.PublishedAt != nil || e.CoverUrl != nil {
		t.Fatalf("unpublished %+v", e)
	}
	expectProblem(t, student.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil), http.StatusNotFound, "showcase.not_found")
	if card := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); card.Visibility != apigen.ProjectVisibilityPrivate {
		t.Fatalf("project visibility %s", card.Visibility)
	}
}

// pageTag is the ETag of a page version.
func pageTag(e apigen.ShowcaseEntry) string { return `"` + strconv.Itoa(e.Version) + `"` }

// publicPage publishes an approved public page of a new project.
func publicPage(t *testing.T, h *harness, owner, manager *browser) (apigen.Project, string) {
	t.Helper()
	p, base := showcaseProject(t, h, owner, "AgroScan")
	expect(t, owner.do(http.MethodPut, base+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(`"0"`)), http.StatusOK)
	e := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPost, base+"/showcase/publish", nil))
	expect(t, manager.do(http.MethodPost, "/api/v1/admin/showcase/"+p.Id.String()+"/review", apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(e))),
		http.StatusOK)
	return p, base
}

func TestShowcaseContacts(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	p, base := publicPage(t, h, owner, manager)
	// The passport's QR code opens the public page.
	expect(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindPassport, Format: apigen.MaterialFormatPdf}),
		http.StatusAccepted)
	if job, ok := h.queue.jobs[len(h.queue.jobs)-1].(incubator.MaterialArgs); !ok || job.PageURL != "http://kazutb.localhost:3000/showcase/agroscan" {
		t.Fatalf("passport job %+v", h.queue.jobs[len(h.queue.jobs)-1])
	}
	partner := h.browser(t, kazutbHost)
	contact := "/api/v1/showcase/entries/agroscan/contact"
	request := apigen.ContactCreate{
		Name: "Ержан Байжанов", Email: "erzhan@agroinvest.kz", Organization: ptr("AgroInvest"), Kind: apigen.ContactKindInvestment,
		Message: "Хотим обсудить пилот на трёх хозяйствах в Костанайской области этой весной.",
	}

	// Bots that fill the hidden field get an answer and nothing is stored.
	trap := request
	trap.Website = ptr("http://spam.example")
	rec := partner.do(http.MethodPost, contact, trap)
	expect(t, rec, http.StatusAccepted)
	h.validate(t, http.MethodPost, contact, rec)
	short := request
	short.Message = "Привет"
	expectProblem(t, partner.do(http.MethodPost, contact, short), http.StatusUnprocessableEntity, "validation.failed")
	if n := len(decode[apigen.ContactRequestList](t, manager.do(http.MethodGet, "/api/v1/admin/contacts", nil)).Items); n != 0 {
		t.Fatalf("stored %d requests", n)
	}

	// By default an innovation manager reads requests first.
	rec = partner.do(http.MethodPost, contact, request, func(r *http.Request) { r.Header.Set("Accept-Language", "ru") })
	expect(t, rec, http.StatusAccepted)
	if got := decode[apigen.ContactAccepted](t, rec); got.Status != apigen.ContactAcceptedStatusPending {
		t.Fatalf("accepted %+v", got)
	}
	if n := len(decode[apigen.ContactRequestList](t, owner.do(http.MethodGet, base+"/contacts", nil)).Items); n != 0 {
		t.Fatalf("team sees %d pending requests", n)
	}
	expectProblem(t, owner.do(http.MethodGet, "/api/v1/admin/contacts", nil), http.StatusForbidden, "rbac.forbidden")
	rec = manager.do(http.MethodGet, "/api/v1/admin/contacts?status=pending", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/contacts", rec)
	pending := decode[apigen.ContactRequestList](t, rec).Items
	if len(pending) != 1 || pending[0].ProjectTitle != "AgroScan" || pending[0].Email != "erzhan@agroinvest.kz" {
		t.Fatalf("pending %+v", pending)
	}
	moderate := "/api/v1/admin/contacts/" + pending[0].Id.String() + "/moderate"
	rec = manager.do(http.MethodPost, moderate, apigen.ContactModeration{Forward: true})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, moderate, rec)
	if c := decode[apigen.ContactRequest](t, rec); c.Status != apigen.ContactStatusForwarded || c.ModeratedAt == nil {
		t.Fatalf("forwarded %+v", c)
	}
	expectProblem(t, manager.do(http.MethodPost, moderate, apigen.ContactModeration{Forward: false}), http.StatusConflict, "contact.already_moderated")
	expectProblem(t, manager.do(http.MethodPost, "/api/v1/admin/contacts/"+uuid.NewString()+"/moderate", apigen.ContactModeration{Forward: true}),
		http.StatusNotFound, "contact.not_found")
	letter := h.lastMail(t, "owner@qadam.test", mail.TemplateContactRequest)
	if letter.Data["project"] != "AgroScan" || letter.Data["sender"] != "Ержан Байжанов (AgroInvest)" || letter.Data["email"] != "erzhan@agroinvest.kz" ||
		letter.Data["kind"] != "инвестиции" || letter.Data["link"] != "http://kazutb.localhost:3000/app/projects/"+p.Id.String()+"/showcase" {
		t.Fatalf("letter %+v", letter)
	}
	rec = owner.do(http.MethodGet, base+"/contacts", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/contacts", rec)
	if n := len(decode[apigen.ContactRequestList](t, rec).Items); n != 1 {
		t.Fatalf("team sees %d requests", n)
	}

	// Without moderation requests go straight to the team.
	err := h.db.Worker.Tx(db.WithSystem(context.Background()), func(ctx context.Context) error {
		_, err := h.db.Worker.Conn(ctx).Exec(ctx, "UPDATE tenant_settings SET contact_moderation = false WHERE tenant_id = $1", ts.kazutb)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(h.mails(t))
	if got := decode[apigen.ContactAccepted](t, partner.do(http.MethodPost, contact, request)); got.Status != apigen.ContactAcceptedStatusForwarded {
		t.Fatalf("direct %+v", got)
	}
	if after := len(h.mails(t)); after != before+1 {
		t.Fatalf("letters %d → %d", before, after)
	}

	// An address writes to a project a few times a day…
	expect(t, partner.do(http.MethodPost, contact, request), http.StatusAccepted)
	expectProblem(t, partner.do(http.MethodPost, contact, request), http.StatusTooManyRequests, "showcase.too_many_requests")
	// …and a sender a few requests an hour.
	other := request
	other.Email = "dina@agroinvest.kz"
	expect(t, partner.do(http.MethodPost, contact, other), http.StatusAccepted)
	other.Email = "aidos@agroinvest.kz"
	expectProblem(t, partner.do(http.MethodPost, contact, other), http.StatusTooManyRequests, "showcase.too_many_requests")
	// A malformed address is a field error, like every other field.
	malformed := request
	malformed.Email = "not an e-mail"
	if p := expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, contact, malformed), http.StatusUnprocessableEntity, "validation.failed"); len(p.Errors) != 1 ||
		p.Errors[0].Field != "email" || p.Errors[0].Code != "email" {
		t.Fatalf("malformed e-mail %+v", p.Errors)
	}

	// The team can close the form.
	closed := pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic)
	closed.ContactEnabled = false
	expect(t, owner.do(http.MethodPut, base+"/showcase", closed, ifMatchHeader(`"1"`)), http.StatusOK)
	expectProblem(t, partner.do(http.MethodPost, contact, request), http.StatusConflict, "showcase.contact_disabled")
}

const testShell = `<!doctype html>
<html lang="kk">
  <head>
    <meta charset="UTF-8" />
    <meta name="description" content="Qadam" />
    <title>Qadam</title>
    <script type="module" src="/assets/index-abc.js"></script>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
`

func TestShowcaseSEO(t *testing.T) {
	shell := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, testShell) }))
	defer shell.Close()
	h := newHarness(t, "QADAM_APP_SHELL_URL="+shell.URL+"/index.html")
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	publicPage(t, h, owner, manager)
	crawler := h.browser(t, kazutbHost)

	rec := crawler.do(http.MethodGet, "/showcase/agroscan", nil)
	expect(t, rec, http.StatusOK)
	page := rec.Body.String()
	for _, want := range []string{
		`<title>AgroScan — ҚазТБУ витринасы</title>`,
		`<meta name="description" content="Болезни пшеницы по фото за 10 секунд" />`,
		`<link rel="canonical" href="http://kazutb.localhost:3000/showcase/agroscan" />`,
		`<link rel="alternate" hreflang="kk" href="http://kazutb.localhost:3000/showcase/agroscan?lang=kk" />`,
		`<link rel="alternate" hreflang="ru" href="http://kazutb.localhost:3000/showcase/agroscan?lang=ru" />`,
		`<link rel="alternate" hreflang="en" href="http://kazutb.localhost:3000/showcase/agroscan?lang=en" />`,
		`<link rel="alternate" hreflang="x-default" href="http://kazutb.localhost:3000/showcase/agroscan" />`,
		`<meta property="og:locale:alternate" content="ru_RU" />`,
		`<meta property="og:image" content="http://kazutb.localhost:3000/api/v1/showcase/entries/agroscan/og.png?v=1" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`"@type":"CreativeWork"`, `<script type="module" src="/assets/index-abc.js"></script>`,
		`<noscript><article><h1>AgroScan</h1>`, `<h2>Шешім</h2>`,
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("page has no %s:\n%s", want, page)
		}
	}
	if strings.Count(page, `name="description"`) != 1 || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatalf("description tags or CSP: %s", rec.Header())
	}
	// The first screen is drawn before the web client, in the visitor's language.
	if !strings.Contains(page, `<div id="boot" lang="kk"`) || !strings.Contains(page, `</div>`+"\n    "+`<div id="root"></div>`) ||
		rec.Header().Get("Vary") != "Accept-Language, Cookie" {
		t.Fatalf("first screen: %s\n%s", rec.Header(), page)
	}
	// The whole page follows the visitor's language: the description too.
	ru := func(r *http.Request) { r.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8") }
	if page = crawler.do(http.MethodGet, "/showcase/agroscan", nil, ru).Body.String(); !strings.Contains(page, `<div id="boot" lang="ru"`) ||
		!strings.Contains(page, `href="/showcase">Витрина</a>`) || !strings.Contains(page, "<title>AgroScan — витрина КазУТБ</title>") ||
		!strings.Contains(page, `<html lang="ru">`) || !strings.Contains(page, `<link rel="canonical" href="http://kazutb.localhost:3000/showcase/agroscan" />`) {
		t.Fatalf("first screen in Russian:\n%s", page)
	}
	// A language version has its own address, its own canonical, and wins
	// over the saved choice and the browser's languages.
	enCookie := func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "qadam_locale", Value: "kk"}) }
	if page = crawler.do(http.MethodGet, "/showcase/agroscan?lang=en", nil, ru, enCookie).Body.String(); !strings.Contains(page, `<div id="boot" lang="en"`) ||
		!strings.Contains(page, "<title>AgroScan — KazUTB showcase</title>") || !strings.Contains(page, `<html lang="en">`) ||
		!strings.Contains(page, `<link rel="canonical" href="http://kazutb.localhost:3000/showcase/agroscan?lang=en" />`) {
		t.Fatalf("English version:\n%s", page)
	}
	rec = crawler.do(http.MethodGet, "/showcase/missing", nil)
	expect(t, rec, http.StatusNotFound)
	if !strings.Contains(rec.Body.String(), `<meta name="robots" content="noindex" />`) || !strings.Contains(rec.Body.String(), "index-abc.js") ||
		strings.Contains(rec.Body.String(), `id="boot"`) {
		t.Fatalf("missing page: %s", rec.Body)
	}
	for _, path := range []string{"/", "/showcase", "/showcase/agroscan", "/robots.txt"} {
		expect(t, crawler.do(http.MethodHead, path, nil), http.StatusOK)
	}
	rec = crawler.do(http.MethodGet, "/", nil, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "qadam_locale", Value: "en"}) })
	expect(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.Contains(body, "<title>Qadam — innovation platform of KazUTB</title>") ||
		!strings.Contains(body, `<link rel="canonical" href="http://kazutb.localhost:3000/" />`) || !strings.Contains(body, `"@type":"WebSite"`) ||
		!strings.Contains(body, `<link rel="alternate" hreflang="ru" href="http://kazutb.localhost:3000/?lang=ru" />`) ||
		!strings.Contains(body, `<div id="boot" lang="en"`) || !strings.Contains(body, `From idea to market — <em class="text-primary-300">one step</em>`) {
		t.Fatalf("landing page: %s", body)
	}
	// Every university shows the same national list: one canonical address,
	// on the platform's site.
	rec = crawler.do(http.MethodGet, "/showcase?scope=national", nil)
	expect(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.Contains(body, "Қазақстан университеттерінің жобалар витринасы") || !strings.Contains(body, `"@type":"CollectionPage"`) ||
		!strings.Contains(body, `<a href="http://kazutb.localhost:3000/showcase/agroscan">AgroScan</a>`) ||
		!strings.Contains(body, `<link rel="canonical" href="http://localhost:3000/showcase?scope=national" />`) ||
		!strings.Contains(body, `<link rel="alternate" hreflang="en" href="http://localhost:3000/showcase?scope=national&amp;lang=en" />`) ||
		!strings.Contains(body, "Қазақстан университеттерінің жобалары</h1>") {
		t.Fatalf("catalog page: %s", body)
	}
	rec = crawler.do(http.MethodGet, "/sitemap.xml", nil)
	expect(t, rec, http.StatusOK)
	if body := rec.Body.String(); !strings.Contains(body, "<loc>http://kazutb.localhost:3000/showcase/agroscan</loc>") ||
		!strings.Contains(body, "<lastmod>") || strings.Contains(body, "scope=national") {
		t.Fatalf("sitemap: %s", body)
	}
	rec = crawler.do(http.MethodGet, "/robots.txt", nil)
	if body := rec.Body.String(); !strings.Contains(body, "Disallow: /app/") || !strings.Contains(body, "Sitemap: http://kazutb.localhost:3000/sitemap.xml") {
		t.Fatalf("robots: %s", body)
	}
	if n := len(decode[struct{ Items []any }](t, h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/showcase", nil)).Items); n != 0 {
		t.Fatalf("sandbox: %d", n)
	}
	expect(t, h.browser(t, "unknown.example").do(http.MethodGet, "/sitemap.xml", nil), http.StatusNotFound)

	// Link previews: drawn plainly without the renderer, through it otherwise
	// and then kept until the page changes.
	og := "/api/v1/showcase/entries/agroscan/og.png"
	rec = crawler.do(http.MethodGet, og, nil)
	expect(t, rec, http.StatusOK)
	img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("plain preview %v %v", err, rec.Header())
	}
	var calls atomic.Int32
	var html string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = r.ParseMultipartForm(10 << 20)
		f, _, err := r.FormFile("files")
		if err != nil || r.URL.Path != "/forms/chromium/screenshot/html" || r.FormValue("width") != "1200" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		raw, _ := io.ReadAll(f)
		html = string(raw)
		_, _ = w.Write([]byte("\x89PNG rendered"))
	}))
	defer conv.Close()
	h.services.Showcase.Gotenberg = gotenberg.New(conv.URL, time.Second)
	for range 2 {
		rec = crawler.do(http.MethodGet, og, nil)
		expect(t, rec, http.StatusOK)
		if rec.Body.String() != "\x89PNG rendered" {
			t.Fatalf("preview %q", rec.Body)
		}
	}
	if calls.Load() != 1 || !strings.Contains(html, "AgroScan") || !strings.Contains(html, "ҚазТБУ") || !strings.Contains(html, "#067A75") {
		t.Fatalf("renderer called %d times with %s", calls.Load(), html)
	}
	expectProblem(t, crawler.do(http.MethodGet, "/api/v1/showcase/entries/missing/og.png", nil), http.StatusNotFound, "showcase.not_found")
}
