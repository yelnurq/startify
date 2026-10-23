// Package imports loads the structure of a university (faculties,
// departments, groups) and its students from CSV or XLSX files. An upload is
// validated row by row into a preview; nothing changes until the
// administrator commits it.
package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/imports/importsdb"
	"github.com/yelnurq/startify/backend/internal/org"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/user"
)

var errNotFound = apperr.NotFound("import.not_found", "the import does not exist")

var studentColumns = []column{
	{key: "email", required: true, aliases: []string{"e-mail", "почта", "электронная почта", "эл. почта", "электрондық пошта", "пошта"}},
	{key: "full_name", required: true, aliases: []string{"name", "фио", "ф.и.о.", "ф. и. о.", "полное имя", "аты-жөні", "аты жөні", "толық аты-жөні"}},
	{key: "group_code", aliases: []string{"group", "группа", "код группы", "топ", "топ коды"}},
	{key: "faculty_code", aliases: []string{"faculty", "факультет", "код факультета", "факультет коды"}},
	{key: "course", aliases: []string{"курс"}},
	{key: "roles", aliases: []string{"role", "роль", "роли", "рөл", "рөлі"}},
}

var structureColumns = []column{
	{key: "faculty_code", required: true, aliases: []string{"faculty", "код факультета", "факультет коды"}},
	{key: "faculty_name_kk", aliases: []string{"факультет (каз)", "название факультета (каз)", "факультет атауы"}},
	{key: "faculty_name_ru", aliases: []string{"факультет (рус)", "название факультета", "название факультета (рус)"}},
	{key: "faculty_name_en", aliases: []string{"факультет (англ)", "название факультета (англ)", "faculty name"}},
	{key: "department_code", aliases: []string{"department", "код кафедры", "кафедра коды"}},
	{key: "department_name_kk", aliases: []string{"кафедра (каз)", "название кафедры (каз)", "кафедра атауы"}},
	{key: "department_name_ru", aliases: []string{"кафедра (рус)", "название кафедры", "название кафедры (рус)"}},
	{key: "department_name_en", aliases: []string{"кафедра (англ)", "название кафедры (англ)", "department name"}},
	{key: "group_code", aliases: []string{"group", "группа", "код группы", "топ", "топ коды"}},
	{key: "study_level", aliases: []string{"level", "уровень", "уровень обучения", "деңгей", "оқу деңгейі"}},
	{key: "admission_year", aliases: []string{"year", "год поступления", "түскен жылы"}},
}

var studyLevels = map[string]string{
	"bachelor": "bachelor", "бакалавриат": "bachelor", "бакалавр": "bachelor",
	"master": "master", "магистратура": "master", "магистр": "master", "магистрант": "master",
	"doctorate": "doctorate", "phd": "doctorate", "докторантура": "doctorate", "доктор": "doctorate",
}

// importableRoles are the roles a file may grant (D-026): the roles of
// students and of the staff who supervise, mentor and judge them.
// Administration, innovation managers, leadership and outside partners are
// appointed in the interface, never by a file.
var importableRoles = []authz.Role{authz.RoleStudent, authz.RoleSupervisor, authz.RoleMentor, authz.RoleJury}

// limitInvitations bounds the invitation letters a university's imports send
// a day, so the platform (and the university's mail server) cannot be used to
// mail arbitrary addresses at will.
var limitInvitations = ratelimit.Limit{Rate: 5000, Burst: 5000, Period: 24 * time.Hour}

// Service imports spreadsheets.
type Service struct {
	db          *db.DB
	org         *org.Service
	users       *user.Service
	auth        *auth.Service
	audit       *audit.Logger
	limiter     *ratelimit.Limiter
	invitations ratelimit.Limit
}

// NewService returns the import service.
func NewService(database *db.DB, orgs *org.Service, users *user.Service, authSvc *auth.Service, auditLog *audit.Logger, limiter *ratelimit.Limiter) *Service {
	return &Service{db: database, org: orgs, users: users, auth: authSvc, audit: auditLog, limiter: limiter, invitations: limitInvitations}
}

// LimitInvitations replaces the daily allowance of invitation letters (tests,
// installations that onboard a very large university at once).
func (s *Service) LimitInvitations(l ratelimit.Limit) { s.invitations = l }

func (s *Service) q(ctx context.Context) *importsdb.Queries { return importsdb.New(s.db.Conn(ctx)) }

func rowError(field, code string, params map[string]any) apigen.FieldError {
	e := apigen.FieldError{Field: field, Code: code}
	if params != nil {
		e.Params = &params
	}
	return e
}

// Create reads an uploaded file and stores its validated preview.
func (s *Service) Create(ctx context.Context, form *multipart.Reader) (apigen.ImportJob, error) {
	p, err := authz.RequireRole(ctx, authz.RoleTenantAdmin)
	if err != nil {
		return apigen.ImportJob{}, err
	}
	fields, part, err := files.ReadForm(form, "file", maxImportBytes)
	if err != nil {
		return apigen.ImportJob{}, err
	}
	kind := apigen.ImportKind(fields["kind"])
	if !kind.Valid() {
		return apigen.ImportJob{}, apperr.Validation(apperr.FieldError{Field: "kind", Code: validate.CodeInvalid})
	}
	t, err := readTable(part.FileName, part.Data)
	if err != nil {
		return apigen.ImportJob{}, err
	}
	columns := studentColumns
	if kind == apigen.ImportKindStructure {
		columns = structureColumns
	}
	index, keys, err := mapHeader(t.header, columns)
	if err != nil {
		return apigen.ImportJob{}, err
	}
	rows := make([]apigen.ImportRow, len(t.rows))
	for i, record := range t.rows {
		rows[i] = apigen.ImportRow{Line: t.lines[i], Values: values(record, index), Errors: []apigen.FieldError{}}
	}

	var job importsdb.ImportJob
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		if err := s.plan(ctx, kind, rows); err != nil {
			return err
		}
		rawRows, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		rawSummary, err := json.Marshal(summarize(rows))
		if err != nil {
			return err
		}
		job, err = s.q(ctx).CreateImportJob(ctx, importsdb.CreateImportJobParams{
			CreatedBy: &p.UserID, Kind: string(kind), FileName: part.FileName,
			Columns: keys, Rows: rawRows, Summary: rawSummary,
		})
		return err
	})
	if err != nil {
		return apigen.ImportJob{}, fmt.Errorf("imports: create: %w", err)
	}
	return jobAPI(job)
}

func (s *Service) plan(ctx context.Context, kind apigen.ImportKind, rows []apigen.ImportRow) error {
	lookup, err := s.org.LoadLookup(ctx)
	if err != nil {
		return err
	}
	if kind == apigen.ImportKindStructure {
		planStructure(rows, lookup)
		return nil
	}
	return s.planStudents(ctx, rows, lookup)
}

func summarize(rows []apigen.ImportRow) apigen.ImportSummary {
	sum := apigen.ImportSummary{Total: len(rows)}
	for _, r := range rows {
		switch {
		case len(r.Errors) > 0:
			sum.Invalid++
		case r.Action == apigen.ImportRowActionCreate:
			sum.Valid++
			sum.ToCreate++
		case r.Action == apigen.ImportRowActionUpdate:
			sum.Valid++
			sum.ToUpdate++
		default:
			sum.Valid++
		}
	}
	return sum
}

// planStructure validates structure rows. Codes defined by earlier rows of
// the same file count as existing.
func planStructure(rows []apigen.ImportRow, lookup org.Lookup) {
	newFaculties := map[string]bool{}
	newDepartments := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		v := r.Values
		created := false
		fc := strings.ToLower(v["faculty_code"])
		switch {
		case fc == "":
			r.Errors = append(r.Errors, rowError("faculty_code", validate.CodeRequired, nil))
		case len(fc) > 32:
			r.Errors = append(r.Errors, rowError("faculty_code", validate.CodeMaxLength, map[string]any{"count": 32}))
		default:
			if _, ok := lookup.Faculties[fc]; !ok && !newFaculties[fc] {
				if v["faculty_name_kk"] == "" && v["faculty_name_ru"] == "" && v["faculty_name_en"] == "" {
					r.Errors = append(r.Errors, rowError("faculty_name_ru", validate.CodeRequired, nil))
				} else {
					newFaculties[fc] = true
					created = true
				}
			}
		}
		if dc := strings.ToLower(v["department_code"]); dc != "" {
			if len(dc) > 32 {
				r.Errors = append(r.Errors, rowError("department_code", validate.CodeMaxLength, map[string]any{"count": 32}))
			} else if _, ok := lookup.Departments[dc]; !ok && !newDepartments[dc] {
				if v["department_name_kk"] == "" && v["department_name_ru"] == "" && v["department_name_en"] == "" {
					r.Errors = append(r.Errors, rowError("department_name_ru", validate.CodeRequired, nil))
				} else {
					newDepartments[dc] = true
					created = true
				}
			}
		}
		if gc := v["group_code"]; gc != "" {
			if len([]rune(gc)) > 40 {
				r.Errors = append(r.Errors, rowError("group_code", validate.CodeMaxLength, map[string]any{"count": 40}))
			} else if _, ok := lookup.Groups[strings.ToLower(gc)]; !ok {
				created = true
			}
			if lv := strings.ToLower(v["study_level"]); lv != "" && studyLevels[lv] == "" {
				r.Errors = append(r.Errors, rowError("study_level", validate.CodeInvalid, nil))
			}
			if e, bad := rangeError("admission_year", v["admission_year"], 1990, 2100); bad {
				r.Errors = append(r.Errors, e)
			}
		}
		switch {
		case len(r.Errors) > 0:
			r.Action = apigen.ImportRowActionSkip
		case created:
			r.Action = apigen.ImportRowActionCreate
		default:
			r.Action = apigen.ImportRowActionUpdate
		}
	}
}

// planStudents validates student rows against the structure and the existing accounts.
func (s *Service) planStudents(ctx context.Context, rows []apigen.ImportRow, lookup org.Lookup) error {
	emails := make([]string, 0, len(rows))
	for i := range rows {
		rows[i].Values["email"] = strings.ToLower(rows[i].Values["email"])
		emails = append(emails, rows[i].Values["email"])
	}
	// Accounts belong to the auth module, memberships to the user module:
	// one lookup in each for the whole file.
	accounts, err := s.auth.Accounts(ctx, emails)
	if err != nil {
		return fmt.Errorf("imports: users: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(accounts))
	for _, id := range accounts {
		ids = append(ids, id)
	}
	briefs, err := s.users.Briefs(ctx, ids)
	if err != nil {
		return fmt.Errorf("imports: members: %w", err)
	}
	members := map[string]bool{}
	known := map[string]bool{}
	for email, id := range accounts {
		_, member := briefs[id]
		known[email] = true
		members[email] = member
	}
	seen := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		v := r.Values
		email := v["email"]
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
			r.Errors = append(r.Errors, rowError("email", "email", nil))
		} else if seen[email] {
			r.Errors = append(r.Errors, rowError("email", "duplicate", nil))
		}
		seen[email] = true
		if n := len([]rune(v["full_name"])); n < 2 {
			r.Errors = append(r.Errors, rowError("full_name", validate.CodeRequired, nil))
		} else if n > 120 {
			r.Errors = append(r.Errors, rowError("full_name", validate.CodeMaxLength, map[string]any{"count": 120}))
		}
		var groupFaculty uuid.UUID
		if gc := strings.ToLower(v["group_code"]); gc != "" {
			g, ok := lookup.Groups[gc]
			if !ok {
				r.Errors = append(r.Errors, rowError("group_code", validate.CodeNotFound, nil))
			} else {
				groupFaculty = g.FacultyID
			}
		}
		if fc := strings.ToLower(v["faculty_code"]); fc != "" {
			f, ok := lookup.Faculties[fc]
			switch {
			case !ok:
				r.Errors = append(r.Errors, rowError("faculty_code", validate.CodeNotFound, nil))
			case groupFaculty != uuid.Nil && f.ID != groupFaculty:
				r.Errors = append(r.Errors, rowError("group_code", validate.CodeInvalid, nil))
			}
		}
		if e, bad := rangeError("course", v["course"], 1, 7); bad {
			r.Errors = append(r.Errors, e)
		}
		if _, ok := parseRoles(v["roles"]); !ok {
			r.Errors = append(r.Errors, rowError("roles", validate.CodeInvalid, nil))
		}
		switch {
		case len(r.Errors) > 0:
			r.Action = apigen.ImportRowActionSkip
		case known[email] && members[email]:
			r.Action = apigen.ImportRowActionUpdate
		default:
			r.Action = apigen.ImportRowActionCreate
		}
	}
	return nil
}

// rangeError checks an optional whole number within [lo, hi].
func rangeError(field, value string, lo, hi int) (apigen.FieldError, bool) {
	if value == "" {
		return apigen.FieldError{}, false
	}
	n, err := strconv.Atoi(value)
	switch {
	case err != nil:
		return rowError(field, validate.CodeInvalid, nil), true
	case n < lo:
		return rowError(field, validate.CodeMin, map[string]any{"value": lo}), true
	case n > hi:
		return rowError(field, validate.CodeMax, map[string]any{"value": hi}), true
	}
	return apigen.FieldError{}, false
}

// parseRoles reads "student", "supervisor, mentor" or "студент"; empty means student.
func parseRoles(s string) ([]string, bool) {
	aliases := map[string]authz.Role{
		"студент": authz.RoleStudent, "білім алушы": authz.RoleStudent,
		"научный руководитель": authz.RoleSupervisor, "руководитель": authz.RoleSupervisor, "жетекші": authz.RoleSupervisor,
		"ментор": authz.RoleMentor, "эксперт": authz.RoleJury, "жюри": authz.RoleJury, "сарапшы": authz.RoleJury,
	}
	if strings.TrimSpace(s) == "" {
		return []string{string(authz.RoleStudent)}, true
	}
	var out []string
	for part := range strings.FieldsFuncSeq(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ';' || r == '|' }) {
		part = strings.TrimSpace(part)
		role := authz.Role(part)
		if a, ok := aliases[part]; ok {
			role = a
		}
		if !slices.Contains(importableRoles, role) {
			return nil, false
		}
		if !slices.Contains(out, string(role)) {
			out = append(out, string(role))
		}
	}
	return out, len(out) > 0
}

// Get returns an import with its preview.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (apigen.ImportJob, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return apigen.ImportJob{}, err
	}
	var job importsdb.ImportJob
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		job, err = s.q(ctx).GetImportJob(ctx, id)
		return err
	})
	if db.IsNoRows(err) {
		return apigen.ImportJob{}, errNotFound
	}
	if err != nil {
		return apigen.ImportJob{}, fmt.Errorf("imports: get: %w", err)
	}
	return jobAPI(job)
}

func jobAPI(j importsdb.ImportJob) (apigen.ImportJob, error) {
	out := apigen.ImportJob{
		Columns: j.Columns, CommittedAt: j.CommittedAt, CreatedAt: j.CreatedAt, FileName: j.FileName,
		Id: j.ID, Kind: apigen.ImportKind(j.Kind), Status: apigen.ImportStatus(j.Status),
	}
	if out.Columns == nil {
		out.Columns = []string{}
	}
	if err := json.Unmarshal(j.Rows, &out.Rows); err != nil {
		return out, fmt.Errorf("imports: rows: %w", err)
	}
	if err := json.Unmarshal(j.Summary, &out.Summary); err != nil {
		return out, fmt.Errorf("imports: summary: %w", err)
	}
	return out, nil
}

// Commit applies the valid rows of a preview. Rows are validated again: the
// data may have changed since the preview.
func (s *Service) Commit(ctx context.Context, id uuid.UUID) (apigen.ImportResult, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return apigen.ImportResult{}, err
	}
	var result apigen.ImportResult
	err := s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		job, err := q.LockImportJob(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if job.Status != string(apigen.ImportStatusPreview) {
			return apperr.Conflict("import.already_committed", "the import was already applied")
		}
		var rows []apigen.ImportRow
		if err := json.Unmarshal(job.Rows, &rows); err != nil {
			return err
		}
		for i := range rows {
			rows[i].Errors = []apigen.FieldError{}
		}
		kind := apigen.ImportKind(job.Kind)
		if err := s.plan(ctx, kind, rows); err != nil {
			return err
		}
		if kind == apigen.ImportKindStudents {
			if err := s.allowInvitations(ctx, rows); err != nil {
				return err
			}
		}
		if kind == apigen.ImportKindStructure {
			result, err = s.applyStructure(ctx, rows)
		} else {
			result, err = s.applyStudents(ctx, rows)
		}
		if err != nil {
			return err
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if err := q.MarkImportCommitted(ctx, importsdb.MarkImportCommittedParams{Result: raw, ID: id}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "import.committed", TargetType: "import", TargetID: id.String(),
			Metadata: map[string]any{
				"kind": job.Kind, "file": job.FileName, "created": result.Created,
				"updated": result.Updated, "invited": result.Invited, "skipped": result.Skipped,
			},
		})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return result, err
		}
		return result, fmt.Errorf("imports: commit: %w", err)
	}
	return result, nil
}

// allowInvitations takes the invitation letters of the rows that create
// members from the university's daily allowance, all or none.
func (s *Service) allowInvitations(ctx context.Context, rows []apigen.ImportRow) error {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return err
	}
	n := 0
	for _, r := range rows {
		if r.Action == apigen.ImportRowActionCreate {
			n++
		}
	}
	return s.limiter.AllowN(ctx, "import-invitations:"+t.ID.String(), s.invitations, n, "import.too_many_invitations")
}

func names(v map[string]string, prefix string) apiconv.Text {
	t := apiconv.Text{}
	for _, l := range []string{"kk", "ru", "en"} {
		if s := v[prefix+"_name_"+l]; s != "" {
			t[l] = s
		}
	}
	return t
}

func (s *Service) applyStructure(ctx context.Context, rows []apigen.ImportRow) (apigen.ImportResult, error) {
	var res apigen.ImportResult
	for _, r := range rows {
		if r.Action == apigen.ImportRowActionSkip {
			res.Skipped++
			continue
		}
		v := r.Values
		facultyID, created, err := s.org.UpsertFaculty(ctx, org.Item{Code: v["faculty_code"], Name: names(v, "faculty")})
		if err != nil {
			return res, err
		}
		var departmentID *uuid.UUID
		if v["department_code"] != "" {
			id, c, err := s.org.UpsertDepartment(ctx, facultyID, org.Item{Code: v["department_code"], Name: names(v, "department")})
			if err != nil {
				return res, err
			}
			departmentID, created = &id, created || c
		}
		if v["group_code"] != "" {
			level := studyLevels[strings.ToLower(v["study_level"])]
			if level == "" {
				level = "bachelor"
			}
			var year *int16
			if n, err := strconv.Atoi(v["admission_year"]); err == nil {
				y := int16(n) //nolint:gosec // validated range 1990-2100
				year = &y
			}
			_, c, err := s.org.UpsertStudyGroup(ctx, org.GroupItem{
				Code: v["group_code"], FacultyID: facultyID, DepartmentID: departmentID, StudyLevel: level, AdmissionYear: year,
			})
			if err != nil {
				return res, err
			}
			created = created || c
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

func (s *Service) applyStudents(ctx context.Context, rows []apigen.ImportRow) (apigen.ImportResult, error) {
	var res apigen.ImportResult
	t, err := tenancy.Require(ctx)
	if err != nil {
		return res, err
	}
	lookup, err := s.org.LoadLookup(ctx)
	if err != nil {
		return res, err
	}
	emails := make([]string, 0, len(rows))
	for _, r := range rows {
		emails = append(emails, r.Values["email"])
	}
	accounts, err := s.auth.Accounts(ctx, emails)
	if err != nil {
		return res, err
	}

	for _, r := range rows {
		if r.Action == apigen.ImportRowActionSkip {
			res.Skipped++
			continue
		}
		v := r.Values
		roles, _ := parseRoles(v["roles"])
		userID, known := accounts[v["email"]]
		if !known {
			if userID, err = s.auth.CreateInvitedAccount(ctx, v["email"], v["full_name"], t.Locale); err != nil {
				return res, err
			}
		}
		if err := s.users.EnsureProfile(ctx, userID, v["full_name"]); err != nil {
			return res, err
		}
		m, isMember, err := s.users.Membership(ctx, userID)
		if err != nil {
			return res, err
		}
		if !isMember {
			if m, err = s.users.Join(ctx, userID, roles, user.StatusInvited, nil); err != nil {
				return res, err
			}
		}
		study := user.Study{FacultyID: m.FacultyID, DepartmentID: m.DepartmentID, GroupID: m.GroupID, StudyLevel: m.StudyLevel, Course: m.Course}
		if g, ok := lookup.Groups[strings.ToLower(v["group_code"])]; ok {
			study.GroupID, study.FacultyID, study.DepartmentID, study.StudyLevel = &g.ID, &g.FacultyID, g.DepartmentID, &g.StudyLevel
		} else if f, ok := lookup.Faculties[strings.ToLower(v["faculty_code"])]; ok && (study.FacultyID == nil || *study.FacultyID != f.ID) {
			study.FacultyID, study.DepartmentID, study.GroupID = &f.ID, nil, nil
		}
		if n, err := strconv.Atoi(v["course"]); err == nil {
			c := int16(n) //nolint:gosec // validated range 1-7
			study.Course = &c
		}
		if err := s.users.SetStudy(ctx, userID, study); err != nil {
			return res, err
		}
		if isMember {
			res.Updated++
			continue
		}
		res.Created++
		if err := s.auth.Invite(ctx, auth.Invitee{UserID: userID, Email: v["email"], FullName: v["full_name"], Locale: t.Locale}); err != nil {
			return res, err
		}
		res.Invited++
	}
	return res, nil
}

// Handler serves the import operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the imports handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// CreateImport implements apigen.StrictServerInterface.
func (h *Handler) CreateImport(ctx context.Context, req apigen.CreateImportRequestObject) (apigen.CreateImportResponseObject, error) {
	job, err := h.svc.Create(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CreateImport201JSONResponse(job), nil
}

// GetImport implements apigen.StrictServerInterface.
func (h *Handler) GetImport(ctx context.Context, req apigen.GetImportRequestObject) (apigen.GetImportResponseObject, error) {
	job, err := h.svc.Get(ctx, req.ImportId)
	if err != nil {
		return nil, err
	}
	return apigen.GetImport200JSONResponse(job), nil
}

// CommitImport implements apigen.StrictServerInterface.
func (h *Handler) CommitImport(ctx context.Context, req apigen.CommitImportRequestObject) (apigen.CommitImportResponseObject, error) {
	res, err := h.svc.Commit(ctx, req.ImportId)
	if err != nil {
		return nil, err
	}
	return apigen.CommitImport200JSONResponse(res), nil
}
