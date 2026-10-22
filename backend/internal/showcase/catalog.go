package showcase

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
	"github.com/yelnurq/startify/backend/internal/tenant"
)

// Listing scopes (see the queries).
const (
	scopeNational = "national"
	scopeMember   = "member"
	scopePublic   = "public"
)

const (
	featuredCount = 6
	topTags       = 20
	maxQuery      = 100
)

var stageOrder = []string{"idea", "market", "product", "launch"}

// member reports whether the request comes from a signed-in member of the
// university of the address.
func member(ctx context.Context, t tenancy.Tenant) bool {
	p, ok := authz.FromContext(ctx)
	return ok && p.TenantID == t.ID
}

func scopeOf(ctx context.Context, national bool) (string, error) {
	t, err := tenancy.Require(ctx)
	switch {
	case err != nil:
		return "", err
	case national:
		return scopeNational, nil
	case member(ctx, t):
		return scopeMember, nil
	}
	return scopePublic, nil
}

// tsQuery turns the words of a search into a prefix query; only letters and
// digits reach the query syntax.
func tsQuery(q string) *string {
	if utf8.RuneCountInString(q) > maxQuery {
		q = string([]rune(q)[:maxQuery])
	}
	var terms []string
	for _, w := range strings.FieldsFunc(strings.ToLower(q), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if utf8.RuneCountInString(w) < 2 {
			continue
		}
		terms = append(terms, w+":*")
		if len(terms) == 8 {
			break
		}
	}
	if len(terms) == 0 {
		return nil
	}
	out := strings.Join(terms, " & ")
	return &out
}

// card is a listed page.
type card = showcasedb.ListLiveRow

// cards describes listed pages with their universities.
func (s *Service) cards(ctx context.Context, rows []card) ([]apigen.ShowcaseCard, error) {
	var ids []uuid.UUID
	for i := range rows {
		if !slices.Contains(ids, rows[i].TenantID) {
			ids = append(ids, rows[i].TenantID)
		}
	}
	universities, err := s.Tenants.Universities(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]apigen.ShowcaseCard, len(rows))
	for i := range rows {
		out[i] = cardAPI(&rows[i], universities[rows[i].TenantID])
	}
	return out, nil
}

func cardAPI(r *card, u tenant.University) apigen.ShowcaseCard {
	c := apigen.ShowcaseCard{
		Slug: r.Slug, Url: u.Origin + "/showcase/" + r.Slug, Title: r.Title, Tagline: r.Tagline,
		Industry: apigen.Industry(r.Industry), Stage: apigen.ShowcaseStage(r.Stage), Year: int(r.Year),
		CoverUrl: apiconv.FileURL(r.CoverFileID), Needs: needsAPI(r.Needs), Tags: r.Tags, University: u.Brief,
	}
	if len(r.Faculty) > 0 {
		f := apiconv.ParseText(r.Faculty).API()
		c.Faculty = &f
	}
	if r.ShowScore {
		c.Score, c.Level = intPtr(r.Score), levelPtr(r.Level)
	}
	if r.PublishedAt != nil {
		c.PublishedAt = *r.PublishedAt
	}
	return c
}

type listCursor struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

func lower(v *string) *string {
	if v == nil {
		return nil
	}
	out := strings.ToLower(strings.TrimSpace(*v))
	if out == "" {
		return nil
	}
	return &out
}

// List returns published pages, the most recently published first.
func (s *Service) List(ctx context.Context, params apigen.ListShowcaseParams) (apigen.ShowcasePage, error) {
	scope, err := scopeOf(ctx, params.Scope != nil && *params.Scope == apigen.ListShowcaseParamsScopeNational)
	if err != nil {
		return apigen.ShowcasePage{}, err
	}
	limit := pagination.Limit(params.Limit)
	args := showcasedb.ListLiveParams{
		Scope: scope, FacultyID: params.Faculty, Tag: lower(params.Tag), RowLimit: int32(limit + 1), //nolint:gosec // bounded by MaxLimit
	}
	if params.Q != nil {
		args.Query = tsQuery(*params.Q)
	}
	if params.Industry != nil {
		v := string(*params.Industry)
		args.Industry = &v
	}
	if params.Stage != nil {
		v := string(*params.Stage)
		args.Stage = &v
	}
	if params.Need != nil {
		v := string(*params.Need)
		args.Need = &v
	}
	if params.Year != nil {
		y := int32(min(max(*params.Year, 0), 9999))
		args.Year = &y
	}
	if params.Cursor != nil {
		var c listCursor
		if err := pagination.Decode(*params.Cursor, &c); err != nil {
			return apigen.ShowcasePage{}, err
		}
		args.BeforeAt, args.BeforeID = &c.At, &c.ID
	}
	var rows []card
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListLive(ctx, args)
		return err
	})
	if err != nil {
		return apigen.ShowcasePage{}, fmt.Errorf("showcase: list: %w", err)
	}
	page := apigen.ShowcasePage{}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		if last.PublishedAt != nil {
			next := pagination.Encode(listCursor{At: *last.PublishedAt, ID: last.ProjectID})
			page.NextCursor = &next
		}
	}
	if page.Items, err = s.cards(ctx, rows); err != nil {
		return apigen.ShowcasePage{}, err
	}
	return page, nil
}

func counted(counts map[string]int, order []string) []apigen.FacetCount {
	out := []apigen.FacetCount{}
	if order == nil {
		for k := range counts {
			order = append(order, k)
		}
		slices.SortFunc(order, func(a, b string) int { return cmp.Or(counts[b]-counts[a], strings.Compare(a, b)) })
	}
	for _, k := range order {
		if counts[k] > 0 {
			out = append(out, apigen.FacetCount{Key: k, Count: counts[k]})
		}
	}
	return out
}

// Facets counts published pages by the values of the filters.
func (s *Service) Facets(ctx context.Context, national bool) (apigen.ShowcaseFacets, error) {
	scope, err := scopeOf(ctx, national)
	if err != nil {
		return apigen.ShowcaseFacets{}, err
	}
	var rows []showcasedb.ListFacetRowsRow
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListFacetRows(ctx, scope)
		return err
	})
	if err != nil {
		return apigen.ShowcaseFacets{}, fmt.Errorf("showcase: facets: %w", err)
	}
	industries, stages, wanted, years, tags := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	faculties := map[uuid.UUID]*apigen.FacultyFacet{}
	for i := range rows {
		r := &rows[i]
		industries[r.Industry]++
		stages[r.Stage]++
		years[strconv.Itoa(int(r.Year))]++
		for _, n := range r.Needs {
			wanted[n]++
		}
		for _, t := range r.Tags {
			tags[t]++
		}
		if r.FacultyID != nil && len(r.Faculty) > 0 {
			f, ok := faculties[*r.FacultyID]
			if !ok {
				f = &apigen.FacultyFacet{Id: *r.FacultyID, Name: apiconv.ParseText(r.Faculty).API()}
				faculties[*r.FacultyID] = f
			}
			f.Count++
		}
	}
	yearKeys := make([]string, 0, len(years))
	for k := range years {
		yearKeys = append(yearKeys, k)
	}
	slices.Sort(yearKeys)
	slices.Reverse(yearKeys)
	out := apigen.ShowcaseFacets{
		Total: len(rows), Industries: counted(industries, nil), Stages: counted(stages, stageOrder),
		Needs: counted(wanted, needs), Years: counted(years, yearKeys), Tags: counted(tags, nil),
		Faculties: []apigen.FacultyFacet{},
	}
	if len(out.Tags) > topTags {
		out.Tags = out.Tags[:topTags]
	}
	for _, f := range faculties {
		out.Faculties = append(out.Faculties, *f)
	}
	slices.SortFunc(out.Faculties, func(a, b apigen.FacultyFacet) int {
		return cmp.Or(b.Count-a.Count, strings.Compare(a.Id.String(), b.Id.String()))
	})
	return out, nil
}

// Highlights gives the live figures and the strongest pages of the
// university for its landing page.
func (s *Service) Highlights(ctx context.Context) (apigen.ShowcaseHighlights, error) {
	scope, err := scopeOf(ctx, false)
	if err != nil {
		return apigen.ShowcaseHighlights{}, err
	}
	out := apigen.ShowcaseHighlights{}
	if out.Counters.Projects, err = s.Projects.Count(ctx); err != nil {
		return out, err
	}
	if out.Counters.Students, err = s.Users.CountStudents(ctx); err != nil {
		return out, err
	}
	if s.ReadyEvaluations != nil {
		if out.Counters.Evaluations, err = s.ReadyEvaluations(ctx); err != nil {
			return out, err
		}
	}
	var featured []showcasedb.ListFeaturedRow
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		n, err := q.CountLive(ctx, scope)
		if err != nil {
			return err
		}
		out.Counters.Published = int(n)
		featured, err = q.ListFeatured(ctx, showcasedb.ListFeaturedParams{Scope: scope, RowLimit: featuredCount})
		return err
	})
	if err != nil {
		return out, fmt.Errorf("showcase: highlights: %w", err)
	}
	rows := make([]card, len(featured))
	for i := range featured {
		rows[i] = card(featured[i])
	}
	if out.Featured, err = s.cards(ctx, rows); err != nil {
		return out, err
	}
	return out, nil
}

// published loads a live page of the university of the address that the
// request may see: public pages for everyone, the others for members.
func (s *Service) published(ctx context.Context, slug string) (showcasedb.ShowcaseEntry, tenancy.Tenant, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return showcasedb.ShowcaseEntry{}, t, err
	}
	var row showcasedb.ShowcaseEntry
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		row, err = s.q(ctx).GetLiveBySlug(ctx, strings.ToLower(slug))
		return err
	})
	switch {
	case err != nil && !db.IsNoRows(err):
		return row, t, fmt.Errorf("showcase: page: %w", err)
	case err != nil, !public(row.Status, row.Visibility) && !member(ctx, t):
		return row, t, errNotFound
	}
	return row, t, nil
}

func entryCard(e *showcasedb.ShowcaseEntry) card {
	return card{
		ProjectID: e.ProjectID, TenantID: e.TenantID, Slug: e.Slug, Title: e.Title, Tagline: e.Tagline, Industry: e.Industry,
		Stage: e.Stage, Faculty: e.Faculty, Year: e.Year, CoverFileID: e.CoverFileID, Needs: e.Needs, Tags: e.Tags,
		ShowScore: e.ShowScore, Score: e.Score, Level: e.Level, PublishedAt: e.PublishedAt,
	}
}

// imageURL is the address of the link preview of a page; the version makes
// social networks fetch it again after a change.
func imageURL(origin string, e *showcasedb.ShowcaseEntry) string {
	return origin + "/api/v1/showcase/entries/" + e.Slug + "/og.png?v=" + strconv.Itoa(int(e.Version))
}

// Page returns the public page of a project.
func (s *Service) Page(ctx context.Context, slug string) (apigen.ShowcaseProject, error) {
	row, t, err := s.published(ctx, slug)
	if err != nil {
		return apigen.ShowcaseProject{}, err
	}
	universities, err := s.Tenants.Universities(ctx, []uuid.UUID{t.ID})
	if err != nil {
		return apigen.ShowcaseProject{}, err
	}
	u := universities[t.ID]
	c := entryCard(&row)
	return apigen.ShowcaseProject{
		Card: cardAPI(&c, u), VideoUrl: row.VideoUrl, Sections: sectionsOf(row.Sections).api(), Team: teamAPI(row.Team),
		ContactEnabled: row.ContactEnabled, ImageUrl: imageURL(u.Origin, &row),
	}, nil
}
