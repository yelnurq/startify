package showcase

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"

	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

// Link previews are 1200×630, the size social networks show in full.
const (
	ogWidth  = 1200
	ogHeight = 630
	// ogVersion changes the hash of every preview when the design changes.
	ogVersion = "og2"
)

//go:embed og.html
var ogHTML string

var ogTemplate = template.Must(template.New("og").Parse(ogHTML))

var hexColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ogData is what a preview shows; its hash decides when to draw it again.
type ogData struct {
	Version     string       `json:"version"`
	Lang        string       `json:"lang"`
	Title       string       `json:"title"`
	TitleLength int          `json:"-"`
	Tagline     string       `json:"tagline"`
	University  string       `json:"university"`
	Color       template.CSS `json:"color"`
	Stage       string       `json:"stage"`
	Industry    string       `json:"industry"`
	Score       int          `json:"score"`
	ScoreLabel  string       `json:"score_label"`
	Level       string       `json:"level"`
	CoverID     string       `json:"cover_id"`
	Cover       template.URL `json:"-"`
	Route       []routeStop  `json:"-"`
}

// routeStop is a stage on the route from idea to launch drawn on previews
// without a cover.
type routeStop struct {
	X, Y, LabelX, LabelY int
	Anchor, Label        string
	Done, Current        bool
}

// routePoints place the four stages along the path of og.html.
var routePoints = [4][2]int{{40, 350}, {200, 250}, {320, 150}, {360, 40}}

func route(stage, locale string) []routeStop {
	current := 0
	for i, key := range stageOrder {
		if key == stage {
			current = i
		}
	}
	out := make([]routeStop, len(stageOrder))
	for i, key := range stageOrder {
		x, y := routePoints[i][0], routePoints[i][1]
		stop := routeStop{X: x, Y: y, Label: stageLabels[key].Best(locale), Done: i < current, Current: i == current}
		// Labels sit below the first stop and to the left of the others.
		stop.LabelX, stop.LabelY, stop.Anchor = x-34, y+8, "end"
		if i == 0 {
			stop.LabelX, stop.LabelY, stop.Anchor = x-8, y-40, "start"
		}
		out[i] = stop
	}
	return out
}

func (d *ogData) hash() string {
	raw, _ := json.Marshal(d)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:12])
}

// Image returns the link preview of a page, drawing it again when the page
// has changed since the stored one.
func (s *Service) Image(ctx context.Context, slug string) ([]byte, error) {
	row, t, err := s.published(ctx, slug)
	if err != nil {
		return nil, err
	}
	universities, err := s.Tenants.Universities(ctx, []uuid.UUID{t.ID})
	if err != nil {
		return nil, err
	}
	u := universities[t.ID]
	brand := u.Brief.Color
	if !hexColor.MatchString(brand) {
		brand = "#00A3C4"
	}
	d := ogData{
		Version: ogVersion, Lang: t.Locale, Title: row.Title, TitleLength: utf8.RuneCountInString(row.Title), Tagline: row.Tagline,
		University: best(u.Brief.ShortName, t.Locale), Color: template.CSS(brand), //nolint:gosec // checked hex colour
		Stage: stageLabels[row.Stage].Best(t.Locale), Industry: industryLabels[row.Industry].Best(t.Locale),
		ScoreLabel: scoreLabel.Best(t.Locale), Route: route(row.Stage, t.Locale),
	}
	if row.ShowScore && row.Score != nil {
		d.Score = int(*row.Score)
		if row.Level != nil {
			d.Level = levelLabels[*row.Level].Best(t.Locale)
		}
	}
	if row.CoverFileID != nil {
		d.CoverID = row.CoverFileID.String()
	}
	hash := d.hash()
	if row.OgFileID != nil && row.OgHash != nil && *row.OgHash == hash {
		if cached, err := s.readFile(ctx, *row.OgFileID); err == nil {
			return cached, nil
		}
	}
	var cover []byte
	if row.CoverFileID != nil {
		if cover, err = s.readFile(ctx, *row.CoverFileID); err != nil {
			slog.WarnContext(ctx, "showcase: cover unavailable", slog.Any("error", err))
		}
	}
	if len(cover) > 0 {
		d.Cover = template.URL("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(cover)) //nolint:gosec // our own JPEG
	}
	var page bytes.Buffer
	if err := ogTemplate.Execute(&page, d); err != nil {
		return nil, fmt.Errorf("showcase: image: %w", err)
	}
	out, err := s.Gotenberg.HTMLToPNG(ctx, page.Bytes(), ogWidth, ogHeight)
	if err != nil {
		// Without the renderer the preview is the brand colour and the
		// cover; it is not stored, so the next request tries again.
		if !errors.Is(err, gotenberg.ErrNotConfigured) {
			slog.WarnContext(ctx, "showcase: preview rendering failed", slog.Any("error", err))
		}
		return plainImage(brand, cover)
	}
	if err := s.storeImage(ctx, row.ProjectID, row.Slug, hash, out); err != nil {
		slog.WarnContext(ctx, "showcase: preview not stored", slog.Any("error", err))
	}
	return out, nil
}

func (s *Service) readFile(ctx context.Context, id uuid.UUID) ([]byte, error) {
	f, err := s.Files.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	body, err := s.Files.Open(ctx, f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	return io.ReadAll(io.LimitReader(body, maxCover))
}

// storeImage keeps the rendered preview; when another request stored the
// same one meanwhile, ours is dropped.
func (s *Service) storeImage(ctx context.Context, projectID uuid.UUID, slug, hash string, data []byte) error {
	f, err := s.Files.Store(ctx, files.Upload{
		Purpose: files.PurposeGenerated, Visibility: files.VisibilityPublic, FileName: slug + ".png", ContentType: "image/png", Data: data,
	})
	if err != nil {
		return err
	}
	return s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		current, err := q.LockEntry(ctx, projectID)
		if err != nil {
			return err
		}
		if current.OgHash != nil && *current.OgHash == hash && current.OgFileID != nil {
			return s.Files.Delete(ctx, f.ID)
		}
		if err := q.SetImage(ctx, showcasedb.SetImageParams{ProjectID: projectID, OgFileID: &f.ID, OgHash: &hash}); err != nil {
			return err
		}
		if current.OgFileID != nil {
			return s.Files.Delete(ctx, *current.OgFileID)
		}
		return nil
	})
}

// plainImage draws a preview without text: the brand colour with the cover.
func plainImage(hexValue string, cover []byte) ([]byte, error) {
	n, _ := strconv.ParseUint(hexValue[1:], 16, 32)
	brand := color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255} //nolint:gosec // bytes of a hex colour
	canvas := image.NewRGBA(image.Rect(0, 0, ogWidth, ogHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 0xFA, G: 0xFB, B: 0xFC, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(0, 0, 18, ogHeight), &image.Uniform{C: brand}, image.Point{}, draw.Src)
	if len(cover) > 0 {
		if img, err := jpeg.Decode(bytes.NewReader(cover)); err == nil {
			// The cover fills the preview next to the brand stripe, cropped
			// to its proportions around the centre.
			dst := image.Rect(18, 0, ogWidth, ogHeight)
			b := img.Bounds()
			h := b.Dx() * dst.Dy() / dst.Dx()
			src := image.Rect(b.Min.X, b.Min.Y+(b.Dy()-h)/2, b.Max.X, b.Min.Y+(b.Dy()-h)/2+h)
			if h > b.Dy() {
				w := b.Dy() * dst.Dx() / dst.Dy()
				src = image.Rect(b.Min.X+(b.Dx()-w)/2, b.Min.Y, b.Min.X+(b.Dx()-w)/2+w, b.Max.Y)
			}
			xdraw.CatmullRom.Scale(canvas, dst, img, src, xdraw.Src, nil)
		}
	} else {
		draw.Draw(canvas, image.Rect(18, ogHeight-120, ogWidth, ogHeight), &image.Uniform{C: brand}, image.Point{}, draw.Src)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("showcase: image: %w", err)
	}
	return out.Bytes(), nil
}

// imageResponse sends a preview; social networks cache it by address.
type imageResponse struct{ data []byte }

func (r imageResponse) VisitGetShowcaseImageResponse(w http.ResponseWriter) error {
	h := w.Header()
	h.Set("Content-Type", "image/png")
	h.Set("Content-Length", strconv.Itoa(len(r.data)))
	h.Set("Cache-Control", "public, max-age=3600")
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(r.data)
	return err
}
