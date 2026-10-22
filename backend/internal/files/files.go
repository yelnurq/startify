// Package files stores uploaded and generated files: contents in S3-compatible
// storage, metadata in PostgreSQL. Uploads are scanned for malware, and
// contents are streamed through the API so that every download passes the
// visibility rules (D-021).
package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/files/filesdb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
)

// Purposes of files.
const (
	PurposeAvatar     = "avatar"
	PurposeTenantLogo = "tenant_logo"
	PurposeAttachment = "attachment"
	PurposeKnowledge  = "knowledge"
)

// Visibilities of files.
const (
	VisibilityPublic  = "public"
	VisibilityTenant  = "tenant"
	VisibilityPrivate = "private"
)

// Scan modes (config.Files.Scan).
const (
	ScanRequired = "required"
	ScanOptional = "optional"
	ScanOff      = "off"
)

var errNotFound = apperr.NotFound("files.not_found", "the file does not exist")

// Service stores and serves files.
type Service struct {
	db       *db.DB
	store    Storage
	scanner  Scanner
	scanMode string
	log      *slog.Logger
}

// NewService returns the file service.
func NewService(database *db.DB, store Storage, cfg config.Files, log *slog.Logger) *Service {
	s := &Service{db: database, store: store, scanMode: cfg.Scan, log: log}
	if cfg.Scan != ScanOff && cfg.ClamAVAddr != "" {
		s.scanner = &ClamAV{Addr: cfg.ClamAVAddr, Timeout: time.Minute}
	}
	return s
}

// WithScanner replaces the malware scanner (tests).
func (s *Service) WithScanner(scanner Scanner) *Service {
	s.scanner = scanner
	return s
}

// Upload is a file to store.
type Upload struct {
	OwnerID     *uuid.UUID
	Purpose     string
	Visibility  string
	FileName    string
	ContentType string
	Data        []byte
}

// Store scans the upload, writes its contents and records it in the current
// tenant (inside the transaction of ctx, if any).
func (s *Service) Store(ctx context.Context, up Upload) (filesdb.File, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return filesdb.File{}, err
	}
	status, err := s.scan(ctx, up.Data)
	if err != nil {
		return filesdb.File{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return filesdb.File{}, fmt.Errorf("files: id: %w", err)
	}
	key := t.ID.String() + "/" + up.Purpose + "/" + id.String()
	sum := sha256.Sum256(up.Data)
	if err := s.store.Put(ctx, key, up.Data, up.ContentType); err != nil {
		return filesdb.File{}, err
	}
	var f filesdb.File
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		f, err = filesdb.New(s.db.Conn(ctx)).CreateFile(ctx, filesdb.CreateFileParams{
			ID:          id,
			OwnerID:     up.OwnerID,
			Purpose:     up.Purpose,
			Visibility:  up.Visibility,
			ObjectKey:   key,
			FileName:    up.FileName,
			ContentType: up.ContentType,
			SizeBytes:   int64(len(up.Data)),
			Sha256:      hex.EncodeToString(sum[:]),
			ScanStatus:  status,
		})
		return err
	})
	if err != nil {
		_ = s.store.Delete(context.WithoutCancel(ctx), key)
		return filesdb.File{}, fmt.Errorf("files: record: %w", err)
	}
	return f, nil
}

func (s *Service) scan(ctx context.Context, data []byte) (string, error) {
	if s.scanner == nil {
		if s.scanMode == ScanRequired {
			return "", apperr.Unavailable("files.scanner_unavailable", "uploads cannot be checked for viruses right now")
		}
		return "not_scanned", nil
	}
	v, err := s.scanner.Scan(ctx, data)
	if err != nil {
		if s.scanMode == ScanRequired {
			return "", apperr.Unavailable("files.scanner_unavailable", "uploads cannot be checked for viruses right now").Wrap(err)
		}
		s.log.WarnContext(ctx, "virus scan skipped", slog.Any("error", err))
		return "not_scanned", nil
	}
	if !v.Clean {
		s.log.WarnContext(ctx, "infected upload rejected", slog.String("signature", v.Signature))
		return "", apperr.New(apperr.KindValidation, "files.infected", "the file contains malware")
	}
	return "clean", nil
}

// Delete marks a file deleted; its contents are purged a day later.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := filesdb.New(s.db.Conn(ctx)).SoftDeleteFile(ctx, id); err != nil {
		return fmt.Errorf("files: delete: %w", err)
	}
	return nil
}

// Metadata returns a file the principal of ctx may read.
func (s *Service) Metadata(ctx context.Context, id uuid.UUID) (filesdb.File, error) {
	var f filesdb.File
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		f, err = filesdb.New(s.db.Conn(ctx)).GetFile(ctx, id)
		return err
	})
	if db.IsNoRows(err) {
		return f, errNotFound
	}
	if err != nil {
		return f, fmt.Errorf("files: get: %w", err)
	}
	if !canRead(ctx, f) {
		// Indistinguishable from a missing file.
		return f, errNotFound
	}
	return f, nil
}

func canRead(ctx context.Context, f filesdb.File) bool {
	if f.Visibility == VisibilityPublic {
		return true
	}
	p, ok := authz.FromContext(ctx)
	if !ok {
		return false
	}
	if p.SuperAdmin {
		return true
	}
	if p.TenantID != f.TenantID {
		return false
	}
	if f.Visibility == VisibilityTenant {
		return true
	}
	return (f.OwnerID != nil && *f.OwnerID == p.UserID) || p.Has(authz.RoleTenantAdmin)
}

// Open streams the contents of a file.
func (s *Service) Open(ctx context.Context, f filesdb.File) (io.ReadCloser, error) {
	body, err := s.store.Get(ctx, f.ObjectKey)
	if errors.Is(err, ErrObjectNotFound) {
		return nil, errNotFound
	}
	return body, err
}

// Part is a file read from a multipart form.
type Part struct {
	FileName    string
	ContentType string
	Data        []byte
}

// ReadForm reads a multipart form with short text fields and one file in
// fileField of at most maxBytes. The content type is sniffed, never trusted.
func ReadForm(mr *multipart.Reader, fileField string, maxBytes int64) (map[string]string, Part, error) {
	fields := map[string]string{}
	var file Part
	found := false
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, file, apperr.BadRequest("request.malformed", "the form cannot be read").Wrap(err)
		}
		name := p.FormName()
		if name == fileField && p.FileName() != "" {
			data, err := io.ReadAll(io.LimitReader(p, maxBytes+1))
			if err != nil {
				return nil, file, apperr.BadRequest("request.malformed", "the file cannot be read").Wrap(err)
			}
			if int64(len(data)) > maxBytes {
				return nil, file, apperr.New(apperr.KindTooLarge, "files.too_large", "the file is too large").
					WithParams(map[string]any{"limit_bytes": maxBytes})
			}
			file = Part{FileName: cleanFileName(p.FileName()), ContentType: http.DetectContentType(data), Data: data}
			found = true
			continue
		}
		value, err := io.ReadAll(io.LimitReader(p, 4<<10))
		if err != nil {
			return nil, file, apperr.BadRequest("request.malformed", "the form cannot be read").Wrap(err)
		}
		fields[name] = strings.TrimSpace(string(value))
	}
	if !found || len(file.Data) == 0 {
		return nil, file, apperr.Validation(apperr.FieldError{Field: fileField, Code: "required"})
	}
	return fields, file, nil
}

func cleanFileName(name string) string {
	name = name[strings.LastIndexAny(name, `/\`)+1:]
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F || r == '"' {
			return -1
		}
		return r
	}, name)
	if len([]rune(name)) > 200 {
		name = string([]rune(name)[:200])
	}
	if name == "" {
		return "file"
	}
	return name
}

// Handler serves file downloads.
type Handler struct {
	svc *Service
}

// NewHandler returns the files handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetFileContent implements apigen.StrictServerInterface.
func (h *Handler) GetFileContent(ctx context.Context, req apigen.GetFileContentRequestObject) (apigen.GetFileContentResponseObject, error) {
	f, err := h.svc.Metadata(ctx, req.FileId)
	if err != nil {
		return nil, err
	}
	tag := `"` + f.Sha256 + `"`
	if r, ok := httpx.RequestFrom(ctx); ok && etagMatches(r.Header.Get("If-None-Match"), tag) {
		return contentResponse{file: f, etag: tag}, nil
	}
	// Downloads may outlive the request timeout; a client disconnect still
	// ends the transfer through the failed write.
	body, err := h.svc.Open(context.WithoutCancel(ctx), f)
	if err != nil {
		return nil, err
	}
	return contentResponse{file: f, etag: tag, body: body}, nil
}

func etagMatches(header, tag string) bool {
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == tag || candidate == "*" {
			return true
		}
	}
	return false
}

// inlineTypes may be displayed by the browser; everything else downloads.
var inlineTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true, "application/pdf": true,
}

type contentResponse struct {
	file filesdb.File
	etag string
	body io.ReadCloser
}

func (c contentResponse) VisitGetFileContentResponse(w http.ResponseWriter) error {
	h := w.Header()
	h.Set("ETag", c.etag)
	if c.file.Visibility == VisibilityPublic {
		// File ids never change contents.
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
		h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	} else {
		h.Set("Cache-Control", "private, max-age=3600")
	}
	if c.body == nil {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	defer func() { _ = c.body.Close() }()

	disposition := "attachment"
	if inlineTypes[c.file.ContentType] {
		disposition = "inline"
	}
	h.Set("Content-Type", c.file.ContentType)
	h.Set("Content-Length", strconv.FormatInt(c.file.SizeBytes, 10))
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": c.file.FileName}))
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, c.body); err != nil {
		// Headers are gone; the client sees a truncated transfer.
		slog.Warn("file download interrupted", slog.String("file_id", c.file.ID.String()), slog.Any("error", err))
	}
	return nil
}

// PurgeArgs schedules removal of files deleted more than a day ago.
type PurgeArgs struct{}

// Kind implements river.JobArgs.
func (PurgeArgs) Kind() string { return "files.purge" }

// InsertOpts deduplicates the job.
func (PurgeArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: 30 * time.Minute}}
}

// PurgeWorker deletes the contents and records of deleted files.
type PurgeWorker struct {
	river.WorkerDefaults[PurgeArgs]
	DB    *db.DB
	Store Storage
}

// Work implements river.Worker.
func (w *PurgeWorker) Work(ctx context.Context, _ *river.Job[PurgeArgs]) error {
	ctx = db.WithSystem(ctx)
	var rows []filesdb.ListPurgeableFilesRow
	err := w.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = filesdb.New(w.DB.Conn(ctx)).ListPurgeableFiles(ctx, 500)
		return err
	})
	if err != nil {
		return fmt.Errorf("files: purge list: %w", err)
	}
	for _, r := range rows {
		if err := w.Store.Delete(ctx, r.ObjectKey); err != nil {
			return err
		}
		err := w.DB.Tx(ctx, func(ctx context.Context) error {
			return filesdb.New(w.DB.Conn(ctx)).PurgeFile(ctx, r.ID)
		})
		if err != nil {
			return fmt.Errorf("files: purge %s: %w", r.ID, err)
		}
	}
	if len(rows) > 0 {
		slog.InfoContext(ctx, "deleted files purged", slog.Int("count", len(rows)))
	}
	return nil
}

// PeriodicJobs returns the purge schedule.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(6*time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return PurgeArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
