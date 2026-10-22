package incubator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/prompts"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/files/filesdb"
	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

var (
	errMaterialNotFound = apperr.NotFound("incubator.material_not_found", "the material does not exist")
	errMaterialNotReady = apperr.Conflict("incubator.material_not_ready", "the material is not ready yet")

	limitMaterials = ratelimit.Limit{Rate: 20, Burst: 10, Period: time.Hour}
)

// ProjectCard is the project as the documents present it, captured when the
// material is requested.
type ProjectCard struct {
	Title    string `json:"title"`
	Tagline  string `json:"tagline"`
	Industry string `json:"industry"`
}

// MaterialArgs schedules the generation of a material.
type MaterialArgs struct {
	TenantID    uuid.UUID   `json:"tenant_id"`
	ProjectID   uuid.UUID   `json:"project_id"`
	MaterialID  uuid.UUID   `json:"material_id"`
	RequestedBy uuid.UUID   `json:"requested_by"`
	Origin      string      `json:"origin"`
	Card        ProjectCard `json:"card"`
	Brand       Brand       `json:"brand"`
}

// Kind implements river.JobArgs.
func (MaterialArgs) Kind() string { return "incubator.material" }

// InsertOpts retries while the model or the converter is unavailable.
func (MaterialArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 3} }

func materialAPI(m *incubatordb.ProjectMaterial) apigen.Material {
	out := apigen.Material{
		Id: m.ID, Kind: apigen.MaterialKind(m.Kind), Format: apigen.MaterialFormat(m.Format), Locale: apigen.Locale(m.Locale),
		Status: apigen.MaterialStatus(m.Status), Error: m.Error, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.Status == "ready" {
		u := "/api/v1/projects/" + m.ProjectID.String() + "/materials/" + m.ID.String() + "/content"
		out.DownloadUrl = &u
	}
	return out
}

// RequestMaterial schedules a one-pager, a pitch deck or a passport.
func (s *Service) RequestMaterial(ctx context.Context, projectID uuid.UUID, body apigen.MaterialCreate) (apigen.Material, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Material{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Material{}, err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.Material{}, err
	}
	var v validate.Errors
	if !body.Kind.Valid() {
		v.Add("kind", validate.CodeInvalid, nil)
	}
	if !body.Format.Valid() || (body.Format == apigen.MaterialFormatPptx && body.Kind != apigen.MaterialKindPitchDeck) {
		v.Add("format", validate.CodeInvalid, nil)
	}
	lang := locale(ctx)
	if body.Locale != nil {
		if !body.Locale.Valid() {
			v.Add("locale", validate.CodeInvalid, nil)
		}
		lang = string(*body.Locale)
	}
	if err := v.Err(); err != nil {
		return apigen.Material{}, err
	}
	if err := s.Limiter.Allow(ctx, "materials:"+projectID.String(), limitMaterials, "incubator.too_many_materials"); err != nil {
		return apigen.Material{}, err
	}
	settings, err := s.Tenants.CurrentSettings(ctx)
	if err != nil {
		return apigen.Material{}, err
	}
	user := a.Principal.UserID
	var row incubatordb.ProjectMaterial
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		var err error
		row, err = s.q(ctx).CreateMaterial(ctx, incubatordb.CreateMaterialParams{
			ProjectID: projectID, Kind: string(body.Kind), Format: string(body.Format), Locale: lang, RequestedBy: &user,
		})
		if err != nil {
			return err
		}
		if err := s.Queue.Enqueue(ctx, MaterialArgs{
			TenantID: t.ID, ProjectID: projectID, MaterialID: row.ID, RequestedBy: user, Origin: t.Origin,
			Card:  ProjectCard{Title: a.Project.Title, Tagline: a.Project.Tagline, Industry: a.Project.Industry},
			Brand: Brand{University: settings.ShortName.Best(lang), Color: settings.PrimaryColor},
		}, nil); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "incubator.material_requested", TargetType: "project", TargetID: projectID.String(),
			Metadata: map[string]any{"kind": body.Kind, "format": body.Format},
		})
	})
	if err != nil {
		return apigen.Material{}, fmt.Errorf("incubator: material: %w", err)
	}
	return materialAPI(&row), nil
}

// Materials lists the generated materials of a project.
func (s *Service) Materials(ctx context.Context, projectID uuid.UUID) (apigen.MaterialList, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.MaterialList{}, err
	}
	out := apigen.MaterialList{Items: []apigen.Material{}}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListMaterials(ctx, projectID)
		for i := range rows {
			out.Items = append(out.Items, materialAPI(&rows[i]))
		}
		return err
	})
	if err != nil {
		return out, fmt.Errorf("incubator: materials: %w", err)
	}
	return out, nil
}

// MaterialFile opens a ready material for download.
func (s *Service) MaterialFile(ctx context.Context, projectID, materialID uuid.UUID) (filesdb.File, io.ReadCloser, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return filesdb.File{}, nil, err
	}
	var m incubatordb.ProjectMaterial
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		m, err = s.q(ctx).GetMaterial(ctx, incubatordb.GetMaterialParams{ID: materialID, ProjectID: projectID})
		return err
	})
	if db.IsNoRows(err) {
		return filesdb.File{}, nil, errMaterialNotFound
	}
	if err != nil {
		return filesdb.File{}, nil, fmt.Errorf("incubator: material: %w", err)
	}
	if m.Status != "ready" || m.FileID == nil {
		return filesdb.File{}, nil, errMaterialNotReady
	}
	f, err := s.Files.Load(ctx, *m.FileID)
	if err != nil {
		return f, nil, err
	}
	// Downloads may outlive the request timeout.
	body, err := s.Files.Open(context.WithoutCancel(ctx), f)
	if err != nil {
		return f, nil, err
	}
	return f, body, nil
}

// materialResponse streams a material as an attachment.
type materialResponse struct {
	file filesdb.File
	body io.ReadCloser
}

func (m materialResponse) VisitGetProjectMaterialContentResponse(w http.ResponseWriter) error {
	defer func() { _ = m.body.Close() }()
	h := w.Header()
	h.Set("Content-Type", m.file.ContentType)
	h.Set("Content-Length", strconv.FormatInt(m.file.SizeBytes, 10))
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": m.file.FileName}))
	h.Set("Cache-Control", "private, no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, m.body)
	return err
}

// MaterialWorker writes the texts with the model and renders the document.
type MaterialWorker struct {
	river.WorkerDefaults[MaterialArgs]
	DB        *db.DB
	Gateway   *ai.Gateway
	Files     *files.Service
	Gotenberg *gotenberg.Client
	// Now is replaceable in tests.
	Now func() time.Time
}

// Timeout leaves room for a slow model and the converter.
func (w *MaterialWorker) Timeout(*river.Job[MaterialArgs]) time.Duration { return 5 * time.Minute }

// Work implements river.Worker.
func (w *MaterialWorker) Work(ctx context.Context, job *river.Job[MaterialArgs]) error {
	args := job.Args
	ctx = tenancy.With(ctx, tenancy.Tenant{ID: args.TenantID, Origin: args.Origin})
	ctx = db.WithTenant(ctx, args.TenantID)
	err := w.generate(ctx, args)
	if err == nil {
		return nil
	}
	if job.Attempt < job.MaxAttempts && !errors.Is(err, gotenberg.ErrNotConfigured) {
		return err
	}
	code := "incubator.material_failed"
	if e, ok := apperr.As(err); ok {
		code = e.Code
	} else if errors.Is(err, gotenberg.ErrNotConfigured) {
		code = "incubator.documents_unavailable"
	}
	slog.ErrorContext(ctx, "material not generated", slog.String("material_id", args.MaterialID.String()), slog.Any("error", err))
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		return incubatordb.New(w.DB.Conn(ctx)).SetMaterialFailed(ctx, incubatordb.SetMaterialFailedParams{ID: args.MaterialID, Error: &code})
	})
}

var slugUnsafe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func fileSlug(title string) string {
	s := strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if r := []rune(s); len(r) > 60 {
		s = string(r[:60])
	}
	if s == "" {
		return "project"
	}
	return s
}

func (w *MaterialWorker) generate(ctx context.Context, args MaterialArgs) error {
	var m incubatordb.ProjectMaterial
	var rows []incubatordb.ProjectStepDatum
	err := w.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := incubatordb.New(w.DB.Conn(ctx))
		var err error
		if m, err = q.GetMaterial(ctx, incubatordb.GetMaterialParams{ID: args.MaterialID, ProjectID: args.ProjectID}); err != nil {
			return err
		}
		rows, err = q.ListSteps(ctx, args.ProjectID)
		return err
	})
	if db.IsNoRows(err) {
		return nil // the project or the material is gone
	}
	if err != nil {
		return fmt.Errorf("incubator: material load: %w", err)
	}
	data, _, err := contextJSON(args.Card.Title, args.Card.Tagline, args.Card.Industry, "", rows)
	if err != nil {
		return err
	}
	system, version, err := prompts.Render("materials", prompts.Materials{Slides: DeckSlides, Locale: m.Locale, ProjectData: data})
	if err != nil {
		return err
	}
	res, err := w.Gateway.Complete(ctx, ai.Call{
		Feature: llm.FeatureGeneration, PromptVersion: version, System: system, Locale: m.Locale, Schema: &pitchSchema,
		MaxTokens: 6000, UserID: args.RequestedBy,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Prepare the materials."}},
	})
	if err != nil {
		return err
	}
	var content PitchContent
	if err := json.Unmarshal([]byte(res.Text), &content); err != nil {
		raw, _ := json.Marshal(res.JSON)
		if err := json.Unmarshal(raw, &content); err != nil {
			return fmt.Errorf("incubator: material content: %w", err)
		}
	}
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	doc := newDocData(args.Card, content, args.Brand, m.Locale, now())
	slug := fileSlug(args.Card.Title)
	var out []byte
	var name, contentType string
	switch {
	case m.Kind == "pitch_deck" && m.Format == "pptx":
		out, err = BuildDeck(args.Card.Title, doc.Tagline, m.Locale, content.Slides, args.Brand)
		name, contentType = slug+"-pitch-deck.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case m.Kind == "pitch_deck":
		out, err = w.pdf(ctx, "deck.html", doc)
		name, contentType = slug+"-pitch-deck.pdf", "application/pdf"
	case m.Kind == "passport":
		if err = w.passport(&doc, args, rows); err == nil {
			out, err = w.pdf(ctx, "passport.html", doc)
		}
		name, contentType = slug+"-passport.pdf", "application/pdf"
	default:
		out, err = w.pdf(ctx, "one_pager.html", doc)
		name, contentType = slug+"-one-pager.pdf", "application/pdf"
	}
	if err != nil {
		return err
	}
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		f, err := w.Files.Store(ctx, files.Upload{
			OwnerID: &args.RequestedBy, Purpose: files.PurposeGenerated, Visibility: files.VisibilityPrivate,
			FileName: name, ContentType: contentType, Data: out,
		})
		if err != nil {
			return err
		}
		return incubatordb.New(w.DB.Conn(ctx)).SetMaterialReady(ctx, incubatordb.SetMaterialReadyParams{ID: args.MaterialID, FileID: &f.ID})
	})
}

func (w *MaterialWorker) pdf(ctx context.Context, name string, d docData) ([]byte, error) {
	html, err := render(name, d)
	if err != nil {
		return nil, err
	}
	return w.Gotenberg.HTMLToPDF(ctx, html, gotenberg.Page{PreferCSSPageSize: true})
}

// passport adds the facts of the passport: progress, team, the QR code.
func (w *MaterialWorker) passport(d *docData, args MaterialArgs, rows []incubatordb.ProjectStepDatum) error {
	tmpl := Default()
	completed := 0
	values := map[string]Values{}
	for i := range rows {
		r := &rows[i]
		if done(r.Status) {
			completed++
		}
		values[r.StepKey] = decodeValues(r.Data)
	}
	total := len(tmpl.Keys())
	d.Progress = fmt.Sprintf(d.L["steps"], completed, total)
	d.Percent = completed * 100 / total
	if members, ok := values["team"]["members"].([]any); ok {
		for _, it := range members {
			row, _ := it.(map[string]any)
			name, _ := row["name"].(string)
			role, _ := row["role"].(string)
			if strings.TrimSpace(name) != "" {
				d.Team = append(d.Team, strings.TrimSpace(strings.TrimSuffix(name+" — "+role, " — ")))
			}
		}
	}
	d.Contact, _ = values["pitch"]["contact"].(string)
	l := d.L
	d.Sections = nil
	for _, s := range []section{
		{l["problem"], d.Content.Problem},
		{l["solution"], d.Content.Solution},
		{l["market"], d.Content.Market},
		{l["business_model"], d.Content.Business},
		{l["traction"], d.Content.Traction},
		{l["ask"], d.Content.Ask},
	} {
		if strings.TrimSpace(s.Text) != "" {
			d.Sections = append(d.Sections, s)
		}
	}
	d.URL = strings.TrimSuffix(args.Origin, "/") + "/app/projects/" + args.ProjectID.String()
	qr, err := qrSVG(d.URL)
	if err != nil {
		return err
	}
	d.QR = qr
	return nil
}
