package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// member creates a member of KazUTB without signing them in.
func member(t *testing.T, h *harness, ts tenants, n int, roles ...string) uuid.UUID {
	t.Helper()
	id, err := seed.Account(context.Background(), h.db.App, ts.kazutb, seed.Member{
		Email: fmt.Sprintf("person%d@qadam.test", n), FullName: fmt.Sprintf("Студент Номер%d", n), Locale: "ru",
		Password: password, Roles: roles, Slug: fmt.Sprintf("person%d", n),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTeamSizeAndHandover(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, ownerID := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	add := func(id uuid.UUID, role apigen.ProjectMemberAddRole) int {
		return owner.parallel(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: id, Role: role})
	}

	// The owner and ten members; two more asked for at once: one place left.
	var mates []uuid.UUID
	for i := range 10 {
		id := member(t, h, ts, i, "student")
		mates = append(mates, id)
		if code := add(id, apigen.ProjectMemberAddRoleMember); code != http.StatusCreated {
			t.Fatalf("member %d: %d", i, code)
		}
	}
	late := []uuid.UUID{member(t, h, ts, 20, "student"), member(t, h, ts, 21, "student")}
	codes := make([]int, len(late))
	// Hold the project row meanwhile: both requests are in flight together,
	// and only a count taken under the project lock waits for the other.
	ctx := context.Background()
	hold, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hold.Close(ctx) }()
	tx, err := hold.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "SELECT id FROM projects WHERE id = $1 FOR UPDATE", p.Id); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i, id := range late {
		wg.Go(func() { codes[i] = add(id, apigen.ProjectMemberAddRoleMember) })
	}
	time.Sleep(300 * time.Millisecond)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if codes[0]+codes[1] != http.StatusCreated+http.StatusConflict || (codes[0] != http.StatusCreated && codes[1] != http.StatusCreated) {
		t.Fatalf("the last place went to %v", codes)
	}
	expectProblem(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: member(t, h, ts, 22, "student"), Role: apigen.ProjectMemberAddRoleMember}),
		http.StatusConflict, "project.team_full")

	// Supervisors do not take the team's places, up to their own limit.
	for i := range 3 {
		if code := add(member(t, h, ts, 30+i, "supervisor"), apigen.ProjectMemberAddRoleSupervisor); code != http.StatusCreated {
			t.Fatalf("supervisor %d: %d", i, code)
		}
	}
	expectProblem(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: member(t, h, ts, 33, "supervisor"), Role: apigen.ProjectMemberAddRoleSupervisor}),
		http.StatusConflict, "project.supervisors_full")

	// A moderator hands over a project only when its owner is gone or blocked.
	owner2 := apigen.ProjectMemberUpdateRoleOwner
	handOver := base + "/members/" + mates[0].String()
	expectProblem(t, manager.do(http.MethodPatch, handOver, apigen.ProjectMemberUpdate{Role: &owner2}), http.StatusConflict, "project.owner_active")
	h.db.Exec(t, "UPDATE memberships SET status = 'suspended' WHERE user_id = '"+ownerID.String()+"'")
	rec := manager.do(http.MethodPatch, handOver, apigen.ProjectMemberUpdate{Role: &owner2})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPatch, handOver, rec)
	project := decode[apigen.Project](t, manager.do(http.MethodGet, base, nil))
	roles := map[uuid.UUID]apigen.ProjectRole{}
	for _, m := range project.Members {
		roles[m.User.Id] = m.Role
	}
	if roles[mates[0]] != apigen.ProjectRoleOwner || roles[ownerID] != apigen.ProjectRoleMember {
		t.Fatalf("roles after the handover %+v", roles)
	}
	if count(auditActions(t, h, p.Id.String()), "project.owner_changed") != 1 {
		t.Fatal("the handover is not recorded")
	}

	// An archived project keeps its team.
	expect(t, manager.do(http.MethodDelete, base, nil), http.StatusNoContent)
	expectProblem(t, manager.do(http.MethodDelete, base+"/members/"+mates[1].String(), nil), http.StatusConflict, "project.archived")
}

func TestProjectsAreRateLimited(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	for i := range 12 {
		rec := owner.do(http.MethodPost, "/api/v1/projects", apigen.ProjectCreate{
			Title: fmt.Sprintf("Проект %d", i), Industry: apigen.IndustryAgritech, IpPolicyVersion: ptr("2026-09"),
		})
		if rec.Code == http.StatusTooManyRequests {
			expectProblem(t, rec, http.StatusTooManyRequests, "rate_limited")
			if i != 10 {
				t.Fatalf("limited after %d projects", i)
			}
			return
		}
		expect(t, rec, http.StatusCreated)
	}
	t.Fatal("projects are not rate-limited")
}
