// Package knowledge is the university's knowledge base: regulations of
// competitions and grants, rules, guides. Documents are split into passages
// with embeddings in pgvector; the AI mentor finds and cites them.
package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/knowledge/knowledgedb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

// Upload limits.
const (
	maxFileBytes = 20 << 20
	maxTextBytes = 4 << 20
	embedBatch   = 32
)

var errNotFound = apperr.NotFound("knowledge.not_found", "the document does not exist")

var managers = []authz.Role{authz.RoleTenantAdmin, authz.RoleInnovationManager}

// Service manages the knowledge base.
type Service struct {
	db      *db.DB
	gateway *ai.Gateway
	files   *files.Service
	queue   jobs.Enqueuer
	audit   *audit.Logger
}

// NewService returns the knowledge service.
func NewService(database *db.DB, gateway *ai.Gateway, fs *files.Service, queue jobs.Enqueuer, auditLog *audit.Logger) *Service {
	return &Service{db: database, gateway: gateway, files: fs, queue: queue, audit: auditLog}
}

func (s *Service) q(ctx context.Context) *knowledgedb.Queries { return knowledgedb.New(s.db.Conn(ctx)) }

func documentAPI(id uuid.UUID, title, status string, chunks, chars int32, errText *string, fileID *uuid.UUID, created, updated time.Time) apigen.KnowledgeDocument {
	return apigen.KnowledgeDocument{
		Id: id, Title: title, Status: apigen.KnowledgeStatus(status), ChunkCount: int(chunks), Characters: int(chars),
		Error: errText, FileUrl: apiconv.FileURL(fileID), CreatedAt: created, UpdatedAt: updated,
	}
}

func fromRow(d knowledgedb.KnowledgeDocument) apigen.KnowledgeDocument {
	return documentAPI(d.ID, d.Title, d.Status, d.ChunkCount, int32(utf8.RuneCountInString(d.Body)), //nolint:gosec // bounded upload
		d.Error, d.FileID, d.CreatedAt, d.UpdatedAt)
}

type upload struct {
	title    string
	text     string
	fileName string
	data     []byte
}

func readUpload(form *multipart.Reader) (upload, error) {
	var u upload
	for {
		p, err := form.NextPart()
		if errors.Is(err, io.EOF) {
			return u, nil
		}
		if err != nil {
			return u, apperr.BadRequest("request.malformed", "the form cannot be read").Wrap(err)
		}
		switch p.FormName() {
		case "file":
			data, err := io.ReadAll(io.LimitReader(p, maxFileBytes+1))
			if err != nil {
				return u, apperr.BadRequest("request.malformed", "the file cannot be read").Wrap(err)
			}
			if len(data) > maxFileBytes {
				return u, apperr.New(apperr.KindTooLarge, "files.too_large", "the file is too large").
					WithParams(map[string]any{"limit_bytes": maxFileBytes})
			}
			u.fileName, u.data = p.FileName(), data
		case "title", "text":
			data, err := io.ReadAll(io.LimitReader(p, maxTextBytes+1))
			if err != nil {
				return u, apperr.BadRequest("request.malformed", "the form cannot be read").Wrap(err)
			}
			if len(data) > maxTextBytes {
				return u, apperr.New(apperr.KindTooLarge, "request.too_large", "the text is too long")
			}
			if p.FormName() == "title" {
				u.title = string(data)
			} else {
				u.text = string(data)
			}
		}
	}
}

// Create adds a document and schedules its indexing.
func (s *Service) Create(ctx context.Context, form *multipart.Reader) (apigen.KnowledgeDocument, error) {
	p, err := authz.RequireRole(ctx, managers...)
	if err != nil {
		return apigen.KnowledgeDocument{}, err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.KnowledgeDocument{}, err
	}
	u, err := readUpload(form)
	if err != nil {
		return apigen.KnowledgeDocument{}, err
	}
	var v validate.Errors
	title := v.Text("title", u.title, 1, 200)
	if len(u.data) == 0 && strings.TrimSpace(u.text) == "" {
		v.Add("file", validate.CodeRequired, nil)
	}
	if err := v.Err(); err != nil {
		return apigen.KnowledgeDocument{}, err
	}
	body := normalize(u.text)
	var fileID *uuid.UUID
	if len(u.data) > 0 {
		if body, err = Extract(u.fileName, u.data); err != nil {
			return apigen.KnowledgeDocument{}, err
		}
		f, err := s.files.Store(ctx, files.Upload{
			OwnerID: &p.UserID, Purpose: files.PurposeKnowledge, Visibility: files.VisibilityTenant,
			FileName: u.fileName, ContentType: contentType(u.fileName), Data: u.data,
		})
		if err != nil {
			return apigen.KnowledgeDocument{}, err
		}
		fileID = &f.ID
	}
	var doc knowledgedb.KnowledgeDocument
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		var err error
		doc, err = s.q(ctx).CreateDocument(ctx, knowledgedb.CreateDocumentParams{
			Title: title, Body: body, FileID: fileID, CreatedBy: &p.UserID,
		})
		if err != nil {
			return err
		}
		if err := s.queue.Enqueue(ctx, IndexArgs{TenantID: t.ID, DocumentID: doc.ID}, nil); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "knowledge.document_added", TargetType: "knowledge_document", TargetID: doc.ID.String()})
	})
	if err != nil {
		return apigen.KnowledgeDocument{}, fmt.Errorf("knowledge: create: %w", err)
	}
	return fromRow(doc), nil
}

func contentType(name string) string {
	switch {
	case strings.HasSuffix(strings.ToLower(name), ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(strings.ToLower(name), ".docx"):
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case strings.HasSuffix(strings.ToLower(name), ".md"):
		return "text/markdown; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}

// List returns documents, newest first.
func (s *Service) List(ctx context.Context, params apigen.ListKnowledgeDocumentsParams) (apigen.KnowledgeDocumentPage, error) {
	if _, err := authz.RequireRole(ctx, managers...); err != nil {
		return apigen.KnowledgeDocumentPage{}, err
	}
	limit, err := pagination.Limit(params.Limit)
	if err != nil {
		return apigen.KnowledgeDocumentPage{}, err
	}
	args := knowledgedb.ListDocumentsParams{RowLimit: int32(limit + 1)} //nolint:gosec // bounded by MaxLimit
	if params.Cursor != nil {
		var before uuid.UUID
		if err := pagination.Decode(*params.Cursor, &before); err != nil {
			return apigen.KnowledgeDocumentPage{}, err
		}
		args.BeforeID = &before
	}
	page := apigen.KnowledgeDocumentPage{Items: []apigen.KnowledgeDocument{}}
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListDocuments(ctx, args)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			next := pagination.Encode(rows[len(rows)-1].ID)
			page.NextCursor = &next
		}
		for i := range rows {
			r := &rows[i]
			page.Items = append(page.Items, documentAPI(r.ID, r.Title, r.Status, r.ChunkCount, r.BodyLength, r.Error, r.FileID, r.CreatedAt, r.UpdatedAt))
		}
		return nil
	})
	if err != nil {
		return page, fmt.Errorf("knowledge: list: %w", err)
	}
	return page, nil
}

// Get returns one document.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (apigen.KnowledgeDocument, error) {
	if _, err := authz.RequireRole(ctx, managers...); err != nil {
		return apigen.KnowledgeDocument{}, err
	}
	var doc knowledgedb.KnowledgeDocument
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		doc, err = s.q(ctx).GetDocument(ctx, id)
		return err
	})
	if db.IsNoRows(err) {
		return apigen.KnowledgeDocument{}, errNotFound
	}
	if err != nil {
		return apigen.KnowledgeDocument{}, fmt.Errorf("knowledge: get: %w", err)
	}
	return fromRow(doc), nil
}

// Delete removes a document with its passages and file.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := authz.RequireRole(ctx, managers...); err != nil {
		return err
	}
	err := s.db.Tx(ctx, func(ctx context.Context) error {
		fileID, err := s.q(ctx).DeleteDocument(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if fileID != nil {
			if err := s.files.Delete(ctx, *fileID); err != nil {
				return err
			}
		}
		return s.audit.Record(ctx, audit.Event{Action: "knowledge.document_removed", TargetType: "knowledge_document", TargetID: id.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return err
		}
		return fmt.Errorf("knowledge: delete: %w", err)
	}
	return nil
}

// Hit is a passage matching a question.
type Hit struct {
	DocumentID uuid.UUID
	Title      string
	Heading    string
	Text       string
	Score      float64
}

// Search finds the passages closest to a question in meaning; when
// embeddings are unavailable it falls back to word matching.
func (s *Service) Search(ctx context.Context, query string, k int) ([]Hit, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []Hit{}, nil
	}
	hits := []Hit{}
	vectors, model, embedErr := s.gateway.Embed(ctx, []string{query})
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if embedErr == nil {
			rows, err := q.SearchChunks(ctx, knowledgedb.SearchChunksParams{
				Query: vectorLiteral(vectors[0]), EmbeddingModel: model, RowLimit: int32(k), //nolint:gosec // small k
			})
			if err != nil {
				return err
			}
			for i := range rows {
				r := &rows[i]
				hits = append(hits, Hit{
					DocumentID: r.DocumentID, Title: r.Title, Heading: deref(r.Heading), Text: r.Content,
					Score: math.Max(0, math.Min(1, 1-r.Distance)),
				})
			}
			if len(hits) > 0 {
				return nil
			}
		}
		rows, err := q.SearchChunksText(ctx, knowledgedb.SearchChunksTextParams{Query: query, RowLimit: int32(k)}) //nolint:gosec // small k
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			hits = append(hits, Hit{
				DocumentID: r.DocumentID, Title: r.Title, Heading: deref(r.Heading), Text: r.Content,
				Score: math.Min(1, r.Rank*10),
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("knowledge: search: %w", err)
	}
	return hits, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// vectorLiteral writes a pgvector text literal.
func vectorLiteral(v []float32) string {
	b := make([]byte, 0, len(v)*10+2)
	b = append(b, '[')
	for i, x := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendFloat(b, float64(x), 'g', 7, 32)
	}
	return string(append(b, ']'))
}

// IndexArgs schedules splitting and embedding a document.
type IndexArgs struct {
	TenantID   uuid.UUID `json:"tenant_id"`
	DocumentID uuid.UUID `json:"document_id"`
}

// Kind implements river.JobArgs.
func (IndexArgs) Kind() string { return "knowledge.index" }

// InsertOpts allows retries while the embedding model is unreachable and
// keeps one queued job per document.
func (IndexArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRetryable,
			rivertype.JobStateRunning, rivertype.JobStateScheduled,
		},
	}}
}

// IndexWorker splits a document into passages and stores their embeddings.
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
	DB      *db.DB
	Gateway *ai.Gateway
}

// Work implements river.Worker.
func (w *IndexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	ctx = tenancy.With(ctx, tenancy.Tenant{ID: job.Args.TenantID})
	ctx = db.WithTenant(ctx, job.Args.TenantID)
	err := w.index(ctx, job.Args.DocumentID)
	if err == nil || errors.Is(err, errNotFound) {
		return nil
	}
	if job.Attempt < job.MaxAttempts {
		return err
	}
	msg := "indexing failed"
	if e, ok := apperr.As(err); ok {
		msg = e.Code
	}
	slog.ErrorContext(ctx, "knowledge document not indexed", slog.String("document_id", job.Args.DocumentID.String()), slog.Any("error", err))
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		return knowledgedb.New(w.DB.Conn(ctx)).SetDocumentStatus(ctx, knowledgedb.SetDocumentStatusParams{
			Status: "failed", ID: job.Args.DocumentID, Error: &msg,
		})
	})
}

func (w *IndexWorker) index(ctx context.Context, id uuid.UUID) error {
	var doc knowledgedb.KnowledgeDocument
	err := w.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		doc, err = knowledgedb.New(w.DB.Conn(ctx)).GetDocument(ctx, id)
		return err
	})
	if db.IsNoRows(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("knowledge: load %s: %w", id, err)
	}
	chunks := Split(doc.Body)
	vectors := make([][]float32, 0, len(chunks))
	model := ""
	for start := 0; start < len(chunks); start += embedBatch {
		batch := chunks[start:min(start+embedBatch, len(chunks))]
		texts := make([]string, len(batch))
		for i, c := range batch {
			texts[i] = strings.TrimSpace(c.Heading + "\n" + c.Text)
		}
		v, m, err := w.Gateway.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("knowledge: embed %s: %w", id, err)
		}
		vectors, model = append(vectors, v...), m
	}
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		q := knowledgedb.New(w.DB.Conn(ctx))
		if err := q.DeleteChunks(ctx, id); err != nil {
			return err
		}
		for i, c := range chunks {
			var heading *string
			if c.Heading != "" {
				heading = &c.Heading
			}
			if err := q.InsertChunk(ctx, knowledgedb.InsertChunkParams{
				DocumentID: id, Ordinal: int32(i), Heading: heading, Content: c.Text,
				Embedding: vectorLiteral(vectors[i]), EmbeddingModel: model,
			}); err != nil {
				return err
			}
		}
		return q.SetDocumentStatus(ctx, knowledgedb.SetDocumentStatusParams{
			Status: "ready", ChunkCount: int32(len(chunks)), ID: id, //nolint:gosec // bounded by document size
		})
	})
}

// failedRetryPause is how long a failed document waits before indexing is
// tried again.
const failedRetryPause = 6 * time.Hour

// ReindexArgs schedules indexing of the documents that have no passages of the
// current embedding model: new installations, a changed model, failures.
type ReindexArgs struct{}

// Kind implements river.JobArgs.
func (ReindexArgs) Kind() string { return "knowledge.reindex" }

// ReindexWorker finds such documents in every university.
type ReindexWorker struct {
	river.WorkerDefaults[ReindexArgs]
	DB      *db.DB
	Gateway *ai.Gateway
	Queue   jobs.Enqueuer
}

// Work implements river.Worker.
func (w *ReindexWorker) Work(ctx context.Context, _ *river.Job[ReindexArgs]) error {
	var tenants []uuid.UUID
	err := w.DB.ReadTx(db.WithSystem(ctx), func(ctx context.Context) error {
		var err error
		tenants, err = knowledgedb.New(w.DB.Conn(ctx)).TenantsWithDocuments(ctx)
		return err
	})
	if err != nil {
		return fmt.Errorf("knowledge: reindex: %w", err)
	}
	for _, id := range tenants {
		if err := w.reindex(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (w *ReindexWorker) reindex(ctx context.Context, tenantID uuid.UUID) error {
	ctx = db.WithTenant(tenancy.With(ctx, tenancy.Tenant{ID: tenantID}), tenantID)
	model, err := w.Gateway.EmbeddingModel(ctx)
	if err != nil {
		// Without an embedding provider search falls back to full text.
		slog.WarnContext(ctx, "knowledge not reindexed: no embedding model", slog.String("tenant_id", tenantID.String()), slog.Any("error", err))
		return nil
	}
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		ids, err := knowledgedb.New(w.DB.Conn(ctx)).ListDocumentsForReindex(ctx, knowledgedb.ListDocumentsForReindexParams{
			EmbeddingModel: model, FailedBefore: time.Now().Add(-failedRetryPause),
		})
		if err != nil {
			return fmt.Errorf("knowledge: reindex list: %w", err)
		}
		for _, id := range ids {
			if err := w.Queue.Enqueue(ctx, IndexArgs{TenantID: tenantID, DocumentID: id}, nil); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			slog.InfoContext(ctx, "knowledge documents scheduled for indexing", slog.Int("count", len(ids)), slog.String("model", model))
		}
		return nil
	})
}

// PeriodicJobs returns the reindexing schedule.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return ReindexArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}

// Handler serves the knowledge operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the knowledge handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListKnowledgeDocuments implements apigen.StrictServerInterface.
func (h *Handler) ListKnowledgeDocuments(ctx context.Context, req apigen.ListKnowledgeDocumentsRequestObject) (apigen.ListKnowledgeDocumentsResponseObject, error) {
	page, err := h.svc.List(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListKnowledgeDocuments200JSONResponse(page), nil
}

// CreateKnowledgeDocument implements apigen.StrictServerInterface.
func (h *Handler) CreateKnowledgeDocument(ctx context.Context, req apigen.CreateKnowledgeDocumentRequestObject) (apigen.CreateKnowledgeDocumentResponseObject, error) {
	doc, err := h.svc.Create(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CreateKnowledgeDocument201JSONResponse(doc), nil
}

// GetKnowledgeDocument implements apigen.StrictServerInterface.
func (h *Handler) GetKnowledgeDocument(ctx context.Context, req apigen.GetKnowledgeDocumentRequestObject) (apigen.GetKnowledgeDocumentResponseObject, error) {
	doc, err := h.svc.Get(ctx, req.DocumentId)
	if err != nil {
		return nil, err
	}
	return apigen.GetKnowledgeDocument200JSONResponse(doc), nil
}

// DeleteKnowledgeDocument implements apigen.StrictServerInterface.
func (h *Handler) DeleteKnowledgeDocument(ctx context.Context, req apigen.DeleteKnowledgeDocumentRequestObject) (apigen.DeleteKnowledgeDocumentResponseObject, error) {
	if err := h.svc.Delete(ctx, req.DocumentId); err != nil {
		return nil, err
	}
	return apigen.DeleteKnowledgeDocument204Response{}, nil
}

// SearchKnowledge implements apigen.StrictServerInterface.
func (h *Handler) SearchKnowledge(ctx context.Context, req apigen.SearchKnowledgeRequestObject) (apigen.SearchKnowledgeResponseObject, error) {
	if _, err := authz.RequireRole(ctx, managers...); err != nil {
		return nil, err
	}
	limit, err := pagination.Limit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	hits, err := h.svc.Search(ctx, req.Params.Q, limit)
	if err != nil {
		return nil, err
	}
	items := make([]apigen.KnowledgeHit, len(hits))
	for i, hit := range hits {
		items[i] = apigen.KnowledgeHit{DocumentId: hit.DocumentID, Title: hit.Title, Excerpt: excerpt(hit.Text, 400), Score: hit.Score}
		if hit.Heading != "" {
			items[i].Heading = &hit.Heading
		}
	}
	return apigen.SearchKnowledge200JSONResponse{Items: items}, nil
}

func excerpt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
