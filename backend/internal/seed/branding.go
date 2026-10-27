package seed

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// The seal and the campus photo of KazUTB, as on the university's site
// mooc.kaztbu.edu.kz (provided by the university for its pages).
var (
	//go:embed assets/kazutb-seal.png
	kazutbSeal []byte
	//go:embed assets/kazutb-campus.webp
	kazutbCampus []byte
)

// Branding gives KazUTB its seal and campus photo. The images go through the
// file service like any upload (scan, checksum, object storage); a university
// that already has them keeps its own.
func Branding(ctx context.Context, database *db.DB, store *files.Service) error {
	var t tenantdb.Tenant
	err := database.ReadTx(db.WithSystem(ctx), func(ctx context.Context) error {
		var err error
		t, err = tenantdb.New(database.Conn(ctx)).GetTenantBySlug(ctx, KazUTB.Slug)
		return err
	})
	if err != nil {
		return fmt.Errorf("seed: branding: %w", err)
	}
	if t.LogoFileID != nil && t.CoverFileID != nil {
		return nil
	}
	scoped := tenancy.With(db.WithTenant(ctx, t.ID), tenancy.Tenant{ID: t.ID, Slug: t.Slug, Locale: t.DefaultLocale})
	upload := func(current *uuid.UUID, purpose, name, contentType string, data []byte) (*uuid.UUID, error) {
		if current != nil {
			return current, nil
		}
		f, err := store.Store(scoped, files.Upload{
			Purpose: purpose, Visibility: files.VisibilityPublic,
			FileName: name, ContentType: contentType, Data: data,
		})
		if err != nil {
			return nil, err
		}
		return &f.ID, nil
	}
	logo, err := upload(t.LogoFileID, files.PurposeTenantLogo, "kazutb-seal.png", "image/png", kazutbSeal)
	if err != nil {
		return fmt.Errorf("seed: branding: seal: %w", err)
	}
	cover, err := upload(t.CoverFileID, files.PurposeTenantCover, "kazutb-campus.webp", "image/webp", kazutbCampus)
	if err != nil {
		return fmt.Errorf("seed: branding: photo: %w", err)
	}
	err = database.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		return tenantdb.New(database.Conn(ctx)).SetTenantBranding(ctx, tenantdb.SetTenantBrandingParams{
			ID: t.ID, LogoFileID: logo, CoverFileID: cover,
		})
	})
	if err != nil {
		return fmt.Errorf("seed: branding: %w", err)
	}
	return nil
}
