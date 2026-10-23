package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator"
)

// materialFile is the stored file of a material, which the API never names.
func materialFile(t *testing.T, h *harness, materialID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var id uuid.UUID
	if err := conn.QueryRow(ctx, "SELECT file_id FROM project_materials WHERE id = $1", materialID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFilesFollowWhatTheyBelongTo(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	mate, mateID := account(t, h, ts, "mate@qadam.test", "Дана Сейітқали", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: mateID, Role: apigen.ProjectMemberAddRoleMember}), http.StatusCreated)

	// A member orders a material, then leaves the team.
	h.queue.jobs = nil
	rec := mate.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf})
	expect(t, rec, http.StatusAccepted)
	m := decode[apigen.Material](t, rec)
	worker := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files, Gotenberg: fakeConverter(t)}
	args := h.queue.jobs[len(h.queue.jobs)-1].(incubator.MaterialArgs)
	if err := worker.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	expect(t, mate.do(http.MethodDelete, base+"/members/"+mateID.String(), nil), http.StatusNoContent)
	file := "/api/v1/files/" + materialFile(t, h, m.Id).String() + "/content"
	expectProblem(t, mate.do(http.MethodGet, file, nil), http.StatusNotFound, "files.not_found")
	expectProblem(t, mate.do(http.MethodGet, base+"/materials/"+m.Id.String()+"/content", nil), http.StatusNotFound, "project.not_found")
	// The team downloads it through the project.
	expect(t, owner.do(http.MethodGet, base+"/materials/"+m.Id.String()+"/content", nil), http.StatusOK)
	expectProblem(t, owner.do(http.MethodGet, file, nil), http.StatusNotFound, "files.not_found")

	// The sources of the knowledge base are for those who keep it.
	text := "Раздел 1. Сроки\n\nЗаявки на конкурс стартап-проектов принимаются до 1 декабря."
	rec = manager.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Положение о конкурсе"}, "file", "polozhenie.txt", []byte(text)))
	expect(t, rec, http.StatusCreated)
	doc := decode[apigen.KnowledgeDocument](t, rec)
	expectProblem(t, owner.do(http.MethodGet, *doc.FileUrl, nil), http.StatusNotFound, "files.not_found")
	expect(t, manager.do(http.MethodGet, *doc.FileUrl, nil), http.StatusOK)
}

func TestPictureUploadsAreRateLimited(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	_, base := showcaseProject(t, h, owner, "AgroScan")
	// Avatars and covers share one allowance.
	for i := range 10 {
		path := "/api/v1/me/avatar"
		if i%2 == 1 {
			path = base + "/showcase/cover"
		}
		expect(t, owner.do(http.MethodPut, path, multipartForm(t, nil, "file", "photo.png", pngImage(t, 480, 300))), http.StatusOK)
	}
	expectProblem(t, owner.do(http.MethodPut, "/api/v1/me/avatar", multipartForm(t, nil, "file", "photo.png", pngImage(t, 480, 300))),
		http.StatusTooManyRequests, "rate_limited")
}
