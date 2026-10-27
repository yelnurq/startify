package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

func TestApprovedPagesAreReviewedAgain(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	student, _ := account(t, h, ts, "reader@qadam.test", "Дана Ермекова", "student")
	visitor := h.browser(t, kazutbHost)
	p, base := publicPage(t, h, owner, manager)
	path := base + "/showcase"
	review := "/api/v1/admin/showcase/" + p.Id.String() + "/review"
	approve := apigen.ShowcaseReview{Approve: true}
	page := "/api/v1/showcase/entries/agroscan"
	expect(t, visitor.do(http.MethodGet, page, nil), http.StatusOK)

	// Settings that change nothing the page shows keep it published.
	entry := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, path, nil))
	quiet := pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic)
	quiet.ContactEnabled = false
	rec := owner.do(http.MethodPut, path, quiet, ifMatchHeader(pageTag(entry)))
	expect(t, rec, http.StatusOK)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Status != apigen.ShowcaseStatusPublished {
		t.Fatalf("settings %+v", entry)
	}

	// A new text goes back to review: inside the university only meanwhile.
	spam := pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic)
	spam.Tagline = "Лучшие кредиты без проверки — пишите в Telegram"
	rec = owner.do(http.MethodPut, path, spam, ifMatchHeader(pageTag(entry)))
	expect(t, rec, http.StatusOK)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Status != apigen.ShowcaseStatusInReview || entry.SubmittedAt == nil || entry.Url == nil {
		t.Fatalf("edited %+v", entry)
	}
	expectProblem(t, visitor.do(http.MethodGet, page, nil), http.StatusNotFound, "showcase.not_found")
	expect(t, student.do(http.MethodGet, page, nil), http.StatusOK)
	if n := len(decode[apigen.ShowcasePage](t, h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/showcase?scope=national", nil)).Items); n != 0 {
		t.Fatalf("national showcase keeps %d edited pages", n)
	}
	if card := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); card.Visibility != apigen.ProjectVisibilityUniversity {
		t.Fatalf("project visibility %s", card.Visibility)
	}

	// The approval names the version the moderator read.
	expectProblem(t, manager.do(http.MethodPost, review, approve), http.StatusPreconditionRequired, "precondition.required")
	seen := entry
	entry = decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPut, path, pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(pageTag(entry))))
	expectProblem(t, manager.do(http.MethodPost, review, approve, ifMatchHeader(pageTag(seen))), http.StatusPreconditionFailed, "precondition.failed")
	rec = manager.do(http.MethodPost, review, approve, ifMatchHeader(pageTag(entry)))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, review, rec)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Status != apigen.ShowcaseStatusPublished || rec.Header().Get("ETag") != pageTag(entry) {
		t.Fatalf("approved %+v %s", entry, rec.Header().Get("ETag"))
	}
	expect(t, visitor.do(http.MethodGet, page, nil), http.StatusOK)

	// A new cover is a change too; the answer carries the new version.
	rec = owner.do(http.MethodPut, path+"/cover", multipartForm(t, nil, "file", "cover.png", coverPNG(t)))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, path+"/cover", rec)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Status != apigen.ShowcaseStatusInReview || rec.Header().Get("ETag") != pageTag(entry) {
		t.Fatalf("cover %+v %s", entry, rec.Header().Get("ETag"))
	}
	// The team saves right after the upload with the version it got back.
	expect(t, owner.do(http.MethodPut, path, pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(rec.Header().Get("ETag"))), http.StatusOK)
	entry = decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, path, nil))
	expect(t, manager.do(http.MethodPost, review, approve, ifMatchHeader(pageTag(entry))), http.StatusOK)

	// A moderator's take-down holds the page: the team cannot put it back.
	rec = manager.do(http.MethodPost, path+"/unpublish", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, path+"/unpublish", rec)
	if entry = decode[apigen.ShowcaseEntry](t, rec); !entry.Held || entry.Status != apigen.ShowcaseStatusDraft {
		t.Fatalf("taken down %+v", entry)
	}
	university := pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity)
	expect(t, owner.do(http.MethodPut, path, university, ifMatchHeader(pageTag(entry))), http.StatusOK)
	rec = owner.do(http.MethodPost, path+"/publish", nil)
	expect(t, rec, http.StatusOK)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Status != apigen.ShowcaseStatusInReview || !entry.Held || entry.PublishedAt != nil {
		t.Fatalf("held page republished %+v", entry)
	}
	expectProblem(t, student.do(http.MethodGet, page, nil), http.StatusNotFound, "showcase.not_found")
	expect(t, owner.do(http.MethodPost, path+"/unpublish", nil), http.StatusOK)
	if entry = decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPost, path+"/publish", nil)); !entry.Held || entry.Status != apigen.ShowcaseStatusInReview {
		t.Fatalf("the team's own take-down lifts the hold: %+v", entry)
	}
	rec = manager.do(http.MethodPost, review, approve, ifMatchHeader(pageTag(entry)))
	expect(t, rec, http.StatusOK)
	if entry = decode[apigen.ShowcaseEntry](t, rec); entry.Held || entry.Status != apigen.ShowcaseStatusPublished || entry.PublishedAt == nil {
		t.Fatalf("approved again %+v", entry)
	}
	expect(t, student.do(http.MethodGet, page, nil), http.StatusOK)

	actions := auditActions(t, h, p.Id.String())
	if count(actions, "showcase.taken_down") != 1 || count(actions, "showcase.approved") != 4 {
		t.Fatalf("audit %v", actions)
	}
}

func TestModeratorsDoNotApproveTheirOwnPages(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	// An innovation manager who also runs a student project.
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student", "innovation_manager")
	p, base := showcaseProject(t, h, owner, "AgroScan")
	expect(t, owner.do(http.MethodPut, base+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(`"0"`)), http.StatusOK)
	entry := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPost, base+"/showcase/publish", nil))
	review := "/api/v1/admin/showcase/" + p.Id.String() + "/review"
	expectProblem(t, owner.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(entry))),
		http.StatusForbidden, "showcase.own_page")
	// Another moderator approves it.
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	expect(t, manager.do(http.MethodPost, review, apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(entry))), http.StatusOK)
}

func TestShowcaseFollowsTheTeam(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	_, mateID := account(t, h, ts, "mate@qadam.test", "Айгерим Сапарова", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	p, base := showcaseProject(t, h, owner, "AgroScan")
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: mateID, Role: apigen.ProjectMemberAddRoleMember, Title: ptr("CTO")}), http.StatusCreated)
	expect(t, owner.do(http.MethodPut, base+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(`"0"`)), http.StatusOK)
	entry := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodPost, base+"/showcase/publish", nil))
	expect(t, manager.do(http.MethodPost, "/api/v1/admin/showcase/"+p.Id.String()+"/review", apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(entry))), http.StatusOK)
	visitor := h.browser(t, kazutbHost)
	page := func() apigen.ShowcaseProject {
		return decode[apigen.ShowcaseProject](t, visitor.do(http.MethodGet, "/api/v1/showcase/entries/agroscan", nil))
	}
	if got := page().Team; len(got) != 2 || got[1].Role != "CTO" {
		t.Fatalf("team %+v", got)
	}

	// Someone who leaves the team leaves the page, its JSON-LD and the
	// national copy, without a new review.
	expect(t, owner.do(http.MethodDelete, base+"/members/"+mateID.String(), nil), http.StatusNoContent)
	if got := page().Team; len(got) != 1 || got[0].Name != "Нұрлан Ахметов" {
		t.Fatalf("team after leaving %+v", got)
	}
	html := visitor.do(http.MethodGet, "/showcase/agroscan", nil).Body.String()
	if strings.Contains(html, "Айгерим") {
		t.Fatalf("the page still names the former member: %s", html)
	}
	if e := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil)); e.Status != apigen.ShowcaseStatusPublished {
		t.Fatalf("status %+v", e)
	}
}

func TestShowcaseGuards(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	student, _ := account(t, h, ts, "reader@qadam.test", "Дана Ермекова", "student")

	// A video link of the incubator that the page may not embed is dropped,
	// also when the page is first saved by a cover upload.
	p, base := showcaseProject(t, h, owner, "AgroScan")
	expect(t, owner.do(http.MethodPatch, base+"/steps/pitch", apigen.StepUpdate{Values: map[string]any{"video_url": "https://evil.example/pitch"}},
		ifMatchHeader(`"0"`)), http.StatusOK)
	if e := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil)); e.VideoUrl != nil {
		t.Fatalf("suggested video %v", *e.VideoUrl)
	}
	rec := owner.do(http.MethodPut, base+"/showcase/cover", multipartForm(t, nil, "file", "cover.png", coverPNG(t)))
	expect(t, rec, http.StatusOK)
	if e := decode[apigen.ShowcaseEntry](t, rec); !e.Saved || e.VideoUrl != nil {
		t.Fatalf("saved by the cover %+v", e)
	}

	// Members-only pages and their previews stay out of shared caches.
	university := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil))
	expect(t, owner.do(http.MethodPut, base+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityUniversity), ifMatchHeader(pageTag(university))), http.StatusOK)
	expect(t, owner.do(http.MethodPost, base+"/showcase/publish", nil), http.StatusOK)
	rec = student.do(http.MethodGet, "/api/v1/showcase/entries/agroscan/og.png", nil)
	expect(t, rec, http.StatusOK)
	if rec.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(rec.Header().Get("Vary"), "Cookie") {
		t.Fatalf("members-only preview: %v", rec.Header())
	}
	rec = student.do(http.MethodGet, "/api/v1/showcase", nil)
	if rec.Header().Get("Cache-Control") != "private, no-cache" || !strings.Contains(rec.Header().Get("Vary"), "Cookie") {
		t.Fatalf("catalog JSON: %v", rec.Header())
	}
	entry := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil))
	expect(t, owner.do(http.MethodPut, base+"/showcase", pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), ifMatchHeader(pageTag(entry))), http.StatusOK)
	entry = decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil))
	expect(t, manager.do(http.MethodPost, "/api/v1/admin/showcase/"+p.Id.String()+"/review", apigen.ShowcaseReview{Approve: true}, ifMatchHeader(pageTag(entry))), http.StatusOK)
	rec = h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/showcase/entries/agroscan/og.png", nil)
	if rec.Header().Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("public preview: %v", rec.Header())
	}

	// A suspended university leaves the national showcase.
	national := func() int {
		return len(decode[apigen.ShowcasePage](t, h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/showcase?scope=national", nil)).Items)
	}
	facets := func() int {
		return decode[apigen.ShowcaseFacets](t, h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/showcase/facets?scope=national", nil)).Total
	}
	if national() != 1 || facets() != 1 {
		t.Fatal("the page is not on the national showcase")
	}
	h.db.Exec(t, "UPDATE tenants SET status = 'suspended' WHERE id = '"+ts.kazutb.String()+"'")
	if national() != 0 || facets() != 0 {
		t.Fatal("a suspended university stays on the national showcase")
	}
	h.db.Exec(t, "UPDATE tenants SET status = 'active' WHERE id = '"+ts.kazutb.String()+"'")

	// Two projects claiming one address at once: the second gets a field error.
	_, otherBase := showcaseProject(t, h, student, "AgroScan Two")
	other, _ := showcaseProject(t, h, owner, "Kiiz Lab")
	claimed := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ctx := db.WithTenant(context.Background(), ts.kazutb)
		done <- h.db.App.Tx(ctx, func(ctx context.Context) error {
			_, err := showcasedb.New(h.db.App.Conn(ctx)).UpsertEntry(ctx, showcasedb.UpsertEntryParams{
				ProjectID: other.Id, Slug: "contested", Visibility: "university", Title: "Kiiz Lab", Industry: "agritech", Stage: "idea",
				Year: int32(time.Now().Year()), Sections: []byte(`{}`), Team: []byte(`[]`), Needs: []string{}, Tags: []string{}, SearchText: "kiiz",
			})
			close(claimed)
			<-release
			return err
		})
	}()
	<-claimed
	result := make(chan int, 1)
	var body string
	go func() {
		rec := student.do(http.MethodPut, otherBase+"/showcase", pageUpdate("contested", apigen.ShowcaseVisibilityUniversity), ifMatchHeader(`"0"`))
		body = rec.Body.String()
		result <- rec.Code
	}()
	// Let the save reach the address index before the other claim commits.
	time.Sleep(300 * time.Millisecond)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if code := <-result; code != http.StatusUnprocessableEntity || !strings.Contains(body, `"field":"slug"`) || !strings.Contains(body, `"code":"taken"`) {
		t.Fatalf("contested address: %d %s", code, body)
	}
}

func TestContactModerationRespectsTheTeam(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	_, base := publicPage(t, h, owner, manager)
	for i, email := range []string{"a@partner.kz", "b@partner.kz", "c@partner.kz"} {
		partner := h.browser(t, kazutbHost)
		expect(t, partner.do(http.MethodPost, "/api/v1/showcase/entries/agroscan/contact", apigen.ContactCreate{
			Name: "Партнёр " + email[:1], Email: email, Kind: apigen.ContactKindPilot,
			Message: "Готовы обсудить пилот на нашей площадке в этом сезоне, номер " + string(rune('1'+i)) + ".",
		}, func(r *http.Request) { r.RemoteAddr = "198.51.100." + string(rune('1'+i)) + ":1234" }), http.StatusAccepted)
	}

	// The moderation list is paged.
	rec := manager.do(http.MethodGet, "/api/v1/admin/contacts?limit=2", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/contacts", rec)
	first := decode[apigen.ContactRequestList](t, rec)
	if len(first.Items) != 2 || first.NextCursor == nil {
		t.Fatalf("first page %+v", first)
	}
	rest := decode[apigen.ContactRequestList](t, manager.do(http.MethodGet, "/api/v1/admin/contacts?limit=2&cursor="+*first.NextCursor, nil))
	if len(rest.Items) != 1 || rest.NextCursor != nil || rest.Items[0].Id == first.Items[1].Id {
		t.Fatalf("second page %+v", rest)
	}

	// Once the team stops accepting requests, nothing more is forwarded.
	entry := decode[apigen.ShowcaseEntry](t, owner.do(http.MethodGet, base+"/showcase", nil))
	closed := pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic)
	closed.ContactEnabled = false
	expect(t, owner.do(http.MethodPut, base+"/showcase", closed, ifMatchHeader(pageTag(entry))), http.StatusOK)
	moderate := "/api/v1/admin/contacts/" + first.Items[0].Id.String() + "/moderate"
	expectProblem(t, manager.do(http.MethodPost, moderate, apigen.ContactModeration{Forward: true}), http.StatusConflict, "showcase.contact_disabled")
	expect(t, manager.do(http.MethodPost, moderate, apigen.ContactModeration{Forward: false}), http.StatusOK)
}
