package app

import (
	"net/http"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/mail"
)

func TestTeamBoard(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	author := h.browser(t, kazutbHost)
	register(t, author, "kamila@qadam.test", "Камила Есенова")
	replier := h.browser(t, kazutbHost)
	register(t, replier, "arman@qadam.test", "Арман Тоқтаров")
	bystander := h.browser(t, kazutbHost)
	register(t, bystander, "zarina@qadam.test", "Зарина Мұхтар")

	rec := author.do(http.MethodPost, "/api/v1/team-requests", apigen.TeamRequestCreate{Title: "ok", Description: "short", RolesNeeded: []apigen.TeamRole{}, Skills: []string{}})
	expectProblem(t, rec, http.StatusUnprocessableEntity, "validation.failed")
	for _, field := range []string{"title", "description", "roles_needed"} {
		if !strings.Contains(rec.Body.String(), `"field":"`+field+`"`) {
			t.Fatalf("no %s error: %s", field, rec.Body)
		}
	}

	create := apigen.TeamRequestCreate{
		Title:       "Сервис бронирования лабораторий",
		Description: "Ищем фронтенд-разработчика и дизайнера: студенты смогут бронировать лаборатории и оборудование онлайн.",
		RolesNeeded: []apigen.TeamRole{apigen.TeamRoleDeveloper, apigen.TeamRoleDesigner},
		Skills:      []string{"React", "Figma"},
	}
	rec = author.do(http.MethodPost, "/api/v1/team-requests", create, func(r *http.Request) { r.Header.Set("Idempotency-Key", "team-request-0001") })
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, "/api/v1/team-requests", rec)
	req := decode[apigen.TeamRequest](t, rec)
	if !req.IsMine || req.Author.FullName != "Камила Есенова" || req.Status != apigen.TeamRequestStatusOpen {
		t.Fatalf("created: %+v", req)
	}
	// A retried request with the same key returns the same request.
	rec = author.do(http.MethodPost, "/api/v1/team-requests", create, func(r *http.Request) { r.Header.Set("Idempotency-Key", "team-request-0001") })
	if replay := decode[apigen.TeamRequest](t, rec); replay.Id != req.Id || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: %+v %v", replay, rec.Header())
	}
	second := create
	second.Title, second.RolesNeeded, second.Skills = "Аналитика посещаемости кампуса", []apigen.TeamRole{apigen.TeamRoleDataScientist}, []string{"Python"}
	second.Description = "Нужен специалист по данным: по журналам пропусков прогнозируем загрузку аудиторий и столовой."
	expect(t, author.do(http.MethodPost, "/api/v1/team-requests", second), http.StatusCreated)

	path := "/api/v1/team-requests/" + req.Id.String()
	rec = replier.do(http.MethodGet, "/api/v1/team-requests?limit=1", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/team-requests", rec)
	page := decode[apigen.TeamRequestPage](t, rec)
	if len(page.Items) != 1 || page.Items[0].IsMine || page.NextCursor == nil {
		t.Fatalf("page: %+v", page)
	}
	rec = replier.do(http.MethodGet, "/api/v1/team-requests?cursor="+*page.NextCursor, nil)
	if items := decode[apigen.TeamRequestPage](t, rec).Items; len(items) != 1 || items[0].Id != req.Id {
		t.Fatalf("next page: %+v", items)
	}
	for query, want := range map[string]int{"?role=designer": 1, "?skill=python": 1, "?q=%D0%BB%D0%B0%D0%B1%D0%BE%D1%80": 1, "?q=100%25": 0} {
		if n := len(decode[apigen.TeamRequestPage](t, replier.do(http.MethodGet, "/api/v1/team-requests"+query, nil)).Items); n != want {
			t.Fatalf("%s: %d results, want %d", query, n, want)
		}
	}

	expectProblem(t, author.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Хочу в свою же команду!"}),
		http.StatusConflict, "team.own_request")
	rec = replier.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Пишу на React два года, могу показать портфолио."})
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, path+"/responses", rec)
	expectProblem(t, replier.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "И ещё раз, на всякий случай."}),
		http.StatusConflict, "team.already_responded")
	// What the request allows comes first, whatever the message says.
	expectProblem(t, author.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Я"}), http.StatusConflict, "team.own_request")
	expectProblem(t, replier.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Ещё"}), http.StatusConflict, "team.already_responded")
	expectProblem(t, bystander.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Коротко"}), http.StatusUnprocessableEntity, "validation.failed")

	letter := h.lastMail(t, "kamila@qadam.test", mail.TemplateTeamResponse)
	if letter.Data["responder"] != "Арман Тоқтаров" || letter.Data["link"] != "http://kazutb.localhost/team/"+req.Id.String() {
		t.Fatalf("notification: %+v", letter)
	}

	// The author sees replies with contacts; others see only their own.
	rec = author.do(http.MethodGet, path+"/responses", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, path+"/responses", rec)
	replies := decode[struct{ Items []apigen.TeamResponse }](t, rec).Items
	if len(replies) != 1 || replies[0].Email == nil || *replies[0].Email != "arman@qadam.test" {
		t.Fatalf("author view: %+v", replies)
	}
	if own := decode[struct{ Items []apigen.TeamResponse }](t, replier.do(http.MethodGet, path+"/responses", nil)).Items; len(own) != 1 || own[0].Email != nil {
		t.Fatalf("replier view: %+v", own)
	}
	if none := decode[struct{ Items []apigen.TeamResponse }](t, bystander.do(http.MethodGet, path+"/responses", nil)).Items; len(none) != 0 {
		t.Fatalf("bystander view: %+v", none)
	}
	got := decode[apigen.TeamRequest](t, replier.do(http.MethodGet, path, nil))
	if !got.HasResponded || got.ResponsesCount != 1 {
		t.Fatalf("counters: %+v", got)
	}

	// Optimistic locking: an edit based on a stale version is refused.
	rec = author.do(http.MethodGet, path, nil)
	h.validate(t, http.MethodGet, path, rec)
	version := rec.Header().Get("ETag")
	if version == "" {
		t.Fatal("no ETag")
	}
	rec = author.do(http.MethodPatch, path, apigen.TeamRequestUpdate{Title: ptr("Сервис бронирования лабораторий и оборудования")},
		func(r *http.Request) { r.Header.Set("If-Match", version) })
	expect(t, rec, http.StatusOK)
	if rec.Header().Get("ETag") == version {
		t.Fatal("ETag must change with the request")
	}
	expectProblem(t, author.do(http.MethodPatch, path, apigen.TeamRequestUpdate{Title: ptr("Старая версия заголовка")},
		func(r *http.Request) { r.Header.Set("If-Match", version) }), http.StatusPreconditionFailed, "precondition.failed")

	// A moderator's edit of someone's request is recorded with its author.
	moderator := seedAdmin(t, h, ts)
	expect(t, moderator.do(http.MethodPatch, path, apigen.TeamRequestUpdate{Title: ptr("Сервис бронирования лабораторий кампуса")}), http.StatusOK)
	if actions := auditActions(t, h, req.Id.String()); count(actions, "team.request_updated") != 2 {
		t.Fatalf("audit %v", actions)
	}

	closed := apigen.TeamRequestStatusClosed
	expectProblem(t, replier.do(http.MethodPatch, path, apigen.TeamRequestUpdate{Status: &closed}), http.StatusForbidden, "team.forbidden")
	rec = author.do(http.MethodPatch, path, apigen.TeamRequestUpdate{Status: &closed})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPatch, path, rec)
	expectProblem(t, bystander.do(http.MethodPost, path+"/responses", apigen.TeamResponseCreate{Message: "Можно присоединиться?"}),
		http.StatusConflict, "team.closed")
	if n := len(decode[apigen.TeamRequestPage](t, bystander.do(http.MethodGet, "/api/v1/team-requests", nil)).Items); n != 1 {
		t.Fatalf("closed request still listed: %d", n)
	}
	if n := len(decode[apigen.TeamRequestPage](t, author.do(http.MethodGet, "/api/v1/team-requests?mine=true", nil)).Items); n != 2 {
		t.Fatalf("mine: %d", n)
	}

	// Another university sees nothing.
	other := h.browser(t, sandboxHost)
	register(t, other, "outsider@qadam.test", "Outsider Person")
	if n := len(decode[apigen.TeamRequestPage](t, other.do(http.MethodGet, "/api/v1/team-requests", nil)).Items); n != 0 {
		t.Fatalf("sandbox sees %d requests", n)
	}
	expectProblem(t, other.do(http.MethodGet, path, nil), http.StatusNotFound, "team.not_found")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/team-requests", nil), http.StatusUnauthorized, "auth.unauthenticated")
}
