// Package org owns the structure of a university: faculties, departments and
// study groups.
package org

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/org/orgdb"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

// Service reads and writes the structure of the current tenant.
type Service struct {
	db *db.DB
}

// NewService returns the structure service.
func NewService(database *db.DB) *Service { return &Service{db: database} }

func (s *Service) q(ctx context.Context) *orgdb.Queries { return orgdb.New(s.db.Conn(ctx)) }

// Faculties lists faculties with their departments, for members (onboarding
// and administration sign in first; visitors have no use for the structure).
func (s *Service) Faculties(ctx context.Context) ([]apigen.Faculty, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return nil, err
	}
	if _, err := authz.Require(ctx); err != nil {
		return nil, err
	}
	var out []apigen.Faculty
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		faculties, err := s.q(ctx).ListFaculties(ctx)
		if err != nil {
			return err
		}
		departments, err := s.q(ctx).ListDepartments(ctx)
		if err != nil {
			return err
		}
		byFaculty := map[uuid.UUID][]apigen.Department{}
		for i := range departments {
			d := &departments[i]
			byFaculty[d.FacultyID] = append(byFaculty[d.FacultyID], apigen.Department{
				Id: d.ID, Code: d.Code, Name: apiconv.ParseText(d.Name).API(),
			})
		}
		out = make([]apigen.Faculty, 0, len(faculties))
		for i := range faculties {
			f := &faculties[i]
			deps := byFaculty[f.ID]
			if deps == nil {
				deps = []apigen.Department{}
			}
			out = append(out, apigen.Faculty{
				Id: f.ID, Code: f.Code, Name: apiconv.ParseText(f.Name).API(), Departments: deps,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("org: faculties: %w", err)
	}
	return out, nil
}

// StudyGroups lists groups filtered by faculty, department or code prefix,
// for members.
func (s *Service) StudyGroups(ctx context.Context, params apigen.ListStudyGroupsParams) ([]apigen.StudyGroup, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return nil, err
	}
	if _, err := authz.Require(ctx); err != nil {
		return nil, err
	}
	var prefix *string
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		v := strings.TrimSpace(*params.Q)
		prefix = &v
	}
	limit, err := pagination.Limit(params.Limit)
	if err != nil {
		return nil, err
	}
	var out []apigen.StudyGroup
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListStudyGroups(ctx, orgdb.ListStudyGroupsParams{
			FacultyID:    params.FacultyId,
			DepartmentID: params.DepartmentId,
			Prefix:       prefix,
			RowLimit:     int32(limit), //nolint:gosec // bounded by MaxLimit
		})
		if err != nil {
			return err
		}
		out = make([]apigen.StudyGroup, 0, len(rows))
		for i := range rows {
			out = append(out, studyGroupAPI(&rows[i]))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("org: study groups: %w", err)
	}
	return out, nil
}

func studyGroupAPI(g *orgdb.StudyGroup) apigen.StudyGroup {
	var year *int
	if g.AdmissionYear != nil {
		y := int(*g.AdmissionYear)
		year = &y
	}
	return apigen.StudyGroup{
		Id:            g.ID,
		Code:          g.Code,
		FacultyId:     g.FacultyID,
		DepartmentId:  g.DepartmentID,
		StudyLevel:    apigen.StudyLevel(g.StudyLevel),
		AdmissionYear: year,
	}
}

// Refs resolves faculty, department and group references in one round trip each.
type Refs struct {
	Faculties   map[uuid.UUID]apigen.FacultyRef
	Departments map[uuid.UUID]apigen.FacultyRef
	Groups      map[uuid.UUID]apigen.StudyGroupRef
}

// LoadRefs fetches the referenced structure items (must run inside a transaction).
func (s *Service) LoadRefs(ctx context.Context, faculties, departments, groups []uuid.UUID) (Refs, error) {
	refs := Refs{
		Faculties:   map[uuid.UUID]apigen.FacultyRef{},
		Departments: map[uuid.UUID]apigen.FacultyRef{},
		Groups:      map[uuid.UUID]apigen.StudyGroupRef{},
	}
	q := s.q(ctx)
	if len(faculties) > 0 {
		rows, err := q.GetFacultiesByIDs(ctx, faculties)
		if err != nil {
			return refs, fmt.Errorf("org: faculties by id: %w", err)
		}
		for i := range rows {
			f := &rows[i]
			refs.Faculties[f.ID] = apigen.FacultyRef{Id: f.ID, Code: f.Code, Name: apiconv.ParseText(f.Name).API()}
		}
	}
	if len(departments) > 0 {
		rows, err := q.GetDepartmentsByIDs(ctx, departments)
		if err != nil {
			return refs, fmt.Errorf("org: departments by id: %w", err)
		}
		for i := range rows {
			d := &rows[i]
			refs.Departments[d.ID] = apigen.FacultyRef{Id: d.ID, Code: d.Code, Name: apiconv.ParseText(d.Name).API()}
		}
	}
	if len(groups) > 0 {
		rows, err := q.GetStudyGroupsByIDs(ctx, groups)
		if err != nil {
			return refs, fmt.Errorf("org: groups by id: %w", err)
		}
		for i := range rows {
			g := &rows[i]
			refs.Groups[g.ID] = apigen.StudyGroupRef{Id: g.ID, Code: g.Code}
		}
	}
	return refs, nil
}

// Placement is a validated place of study.
type Placement struct {
	FacultyID    *uuid.UUID
	DepartmentID *uuid.UUID
	GroupID      *uuid.UUID
	StudyLevel   *string
}

// ValidatePlacement checks that the department and group belong to the
// faculty, and fills the faculty, department and level from the group when
// omitted. Problems are added to v; the error reports only failures to check.
func (s *Service) ValidatePlacement(ctx context.Context, p Placement, v *validate.Errors) (Placement, error) {
	q := s.q(ctx)
	if p.GroupID != nil {
		g, err := q.GetStudyGroup(ctx, *p.GroupID)
		switch {
		case db.IsNoRows(err):
			v.Add("group_id", validate.CodeNotFound, nil)
		case err != nil:
			return p, fmt.Errorf("org: group: %w", err)
		default:
			if p.FacultyID == nil {
				p.FacultyID = &g.FacultyID
			} else if *p.FacultyID != g.FacultyID {
				v.Add("group_id", validate.CodeInvalid, nil)
			}
			if p.DepartmentID == nil && g.DepartmentID != nil {
				p.DepartmentID = g.DepartmentID
			}
			if p.StudyLevel == nil {
				p.StudyLevel = &g.StudyLevel
			}
		}
	}
	if p.DepartmentID != nil {
		d, err := q.GetDepartment(ctx, *p.DepartmentID)
		switch {
		case db.IsNoRows(err):
			v.Add("department_id", validate.CodeNotFound, nil)
		case err != nil:
			return p, fmt.Errorf("org: department: %w", err)
		case p.FacultyID == nil:
			p.FacultyID = &d.FacultyID
		case *p.FacultyID != d.FacultyID:
			v.Add("department_id", validate.CodeInvalid, nil)
		}
	}
	if p.FacultyID != nil {
		if _, err := q.GetFaculty(ctx, *p.FacultyID); db.IsNoRows(err) {
			v.Add("faculty_id", validate.CodeNotFound, nil)
		} else if err != nil {
			return p, fmt.Errorf("org: faculty: %w", err)
		}
	}
	return p, nil
}

// Item is a structure element to create or update by its code.
type Item struct {
	Code string
	Name apiconv.Text
}

// UpsertFaculty creates or renames a faculty; the bool reports creation.
func (s *Service) UpsertFaculty(ctx context.Context, item Item) (uuid.UUID, bool, error) {
	q := s.q(ctx)
	_, err := q.FindFacultyByCode(ctx, item.Code)
	created := db.IsNoRows(err)
	if err != nil && !created {
		return uuid.Nil, false, fmt.Errorf("org: find faculty: %w", err)
	}
	row, err := q.UpsertFaculty(ctx, orgdb.UpsertFacultyParams{Code: item.Code, Name: item.Name.JSON()})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("org: upsert faculty: %w", err)
	}
	return row.ID, created, nil
}

// UpsertDepartment creates or updates a department of a faculty.
func (s *Service) UpsertDepartment(ctx context.Context, facultyID uuid.UUID, item Item) (uuid.UUID, bool, error) {
	q := s.q(ctx)
	_, err := q.FindDepartmentByCode(ctx, item.Code)
	created := db.IsNoRows(err)
	if err != nil && !created {
		return uuid.Nil, false, fmt.Errorf("org: find department: %w", err)
	}
	row, err := q.UpsertDepartment(ctx, orgdb.UpsertDepartmentParams{
		FacultyID: facultyID, Code: item.Code, Name: item.Name.JSON(),
	})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("org: upsert department: %w", err)
	}
	return row.ID, created, nil
}

// GroupItem is a study group to create or update by its code.
type GroupItem struct {
	Code          string
	FacultyID     uuid.UUID
	DepartmentID  *uuid.UUID
	StudyLevel    string
	AdmissionYear *int16
}

// UpsertStudyGroup creates or updates a group.
func (s *Service) UpsertStudyGroup(ctx context.Context, g GroupItem) (uuid.UUID, bool, error) {
	q := s.q(ctx)
	_, err := q.FindStudyGroupByCode(ctx, g.Code)
	created := db.IsNoRows(err)
	if err != nil && !created {
		return uuid.Nil, false, fmt.Errorf("org: find group: %w", err)
	}
	row, err := q.UpsertStudyGroup(ctx, orgdb.UpsertStudyGroupParams{
		FacultyID: g.FacultyID, DepartmentID: g.DepartmentID, Code: g.Code,
		StudyLevel: g.StudyLevel, AdmissionYear: g.AdmissionYear,
	})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("org: upsert group: %w", err)
	}
	return row.ID, created, nil
}

// Lookup finds structure items by code (for imports).
type Lookup struct {
	Faculties   map[string]orgdb.Faculty
	Departments map[string]orgdb.Department
	Groups      map[string]orgdb.StudyGroup
}

// LoadLookup loads the whole structure keyed by lower-cased code.
func (s *Service) LoadLookup(ctx context.Context) (Lookup, error) {
	q := s.q(ctx)
	l := Lookup{Faculties: map[string]orgdb.Faculty{}, Departments: map[string]orgdb.Department{}, Groups: map[string]orgdb.StudyGroup{}}
	faculties, err := q.ListFaculties(ctx)
	if err != nil {
		return l, fmt.Errorf("org: lookup faculties: %w", err)
	}
	for i := range faculties {
		l.Faculties[strings.ToLower(faculties[i].Code)] = faculties[i]
	}
	departments, err := q.ListDepartments(ctx)
	if err != nil {
		return l, fmt.Errorf("org: lookup departments: %w", err)
	}
	for i := range departments {
		l.Departments[strings.ToLower(departments[i].Code)] = departments[i]
	}
	groups, err := q.ListStudyGroups(ctx, orgdb.ListStudyGroupsParams{RowLimit: 1_000_000})
	if err != nil {
		return l, fmt.Errorf("org: lookup groups: %w", err)
	}
	for i := range groups {
		l.Groups[strings.ToLower(groups[i].Code)] = groups[i]
	}
	return l, nil
}

// Handler serves the structure operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the structure handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListFaculties implements apigen.StrictServerInterface.
func (h *Handler) ListFaculties(ctx context.Context, _ apigen.ListFacultiesRequestObject) (apigen.ListFacultiesResponseObject, error) {
	items, err := h.svc.Faculties(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListFaculties200JSONResponse{Items: items}, nil
}

// ListStudyGroups implements apigen.StrictServerInterface.
func (h *Handler) ListStudyGroups(ctx context.Context, req apigen.ListStudyGroupsRequestObject) (apigen.ListStudyGroupsResponseObject, error) {
	items, err := h.svc.StudyGroups(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListStudyGroups200JSONResponse{Items: items}, nil
}
