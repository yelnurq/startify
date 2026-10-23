package showcase

import (
	"context"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	qmail "github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

// Contact request statuses.
const (
	ContactPending   = "pending"
	ContactForwarded = "forwarded"
	ContactRejected  = "rejected"
)

var (
	errContactDisabled  = apperr.Conflict("showcase.contact_disabled", "the team does not accept requests")
	errContactNotFound  = apperr.NotFound("contact.not_found", "the request does not exist")
	errAlreadyModerated = apperr.Conflict("contact.already_moderated", "the request has already been moderated")

	// A partner writes a few letters an hour, and to one project a few a
	// day. The cap per project only bounds what a flood can store: it is far
	// above what a project receives, so a flood from many addresses cannot
	// shut the form for real partners for a whole day.
	limitContactSender  = ratelimit.PerHour(5)
	limitContactEmail   = ratelimit.Limit{Rate: 3, Burst: 3, Period: 24 * time.Hour}
	limitContactProject = ratelimit.Limit{Rate: 100, Burst: 100, Period: 24 * time.Hour}

	contactKinds = []string{"investment", "partnership", "pilot", "mentoring", "hiring", "other"}
)

// kindLabels name the topics of requests in letters.
var kindLabels = map[string]map[string]string{
	"investment":  {"kk": "инвестиция", "ru": "инвестиции", "en": "investment"},
	"partnership": {"kk": "серіктестік", "ru": "партнёрство", "en": "partnership"},
	"pilot":       {"kk": "пилоттық жоба", "ru": "пилотный проект", "en": "a pilot"},
	"mentoring":   {"kk": "тәлімгерлік", "ru": "менторство", "en": "mentoring"},
	"hiring":      {"kk": "жұмысқа шақыру", "ru": "приглашение на работу", "en": "hiring"},
	"other":       {"kk": "басқа", "ru": "другое", "en": "other"},
}

func contactAPI(c *showcasedb.ContactRequest, projectTitle string) apigen.ContactRequest {
	return apigen.ContactRequest{
		Id: c.ID, ProjectId: c.ProjectID, ProjectTitle: projectTitle, Name: c.Name, Email: c.Email,
		Organization: c.Organization, Kind: apigen.ContactKind(c.Kind), Message: c.Message,
		Status: apigen.ContactStatus(c.Status), CreatedAt: c.CreatedAt, ModeratedAt: c.ModeratedAt,
	}
}

func requestLocale(ctx context.Context, t tenancy.Tenant) string {
	if l := httpx.Locale(ctx); l != "" {
		return l
	}
	if t.Locale != "" {
		return t.Locale
	}
	return "ru"
}

// Contact records a partner's request to a team. Depending on the
// university's setting it waits for an innovation manager or goes to the
// team at once; the team's addresses are never shown to the partner.
func (s *Service) Contact(ctx context.Context, slug string, body apigen.ContactCreate) (apigen.ContactAccepted, error) {
	row, t, err := s.published(ctx, slug)
	if err != nil {
		return apigen.ContactAccepted{}, err
	}
	if !row.ContactEnabled {
		return apigen.ContactAccepted{}, errContactDisabled
	}
	// A filled hidden field means a bot: the request looks accepted and is dropped.
	if body.Website != nil && strings.TrimSpace(*body.Website) != "" {
		return apigen.ContactAccepted{Status: apigen.ContactAcceptedStatusPending}, nil
	}
	var v validate.Errors
	name := v.Text("name", body.Name, 2, 120)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if a, err := mail.ParseAddress(email); err != nil || a.Address != email || len(email) > 254 {
		v.Add("email", "email", nil)
	}
	organization := v.Optional("organization", body.Organization, 160, false)
	if !slices.Contains(contactKinds, string(body.Kind)) {
		v.Add("kind", validate.CodeInvalid, nil)
	}
	message := v.Paragraph("message", body.Message, 20, 2000)
	if err := v.Err(); err != nil {
		return apigen.ContactAccepted{}, err
	}
	sender := email
	if ip := httpx.ClientIPFrom(ctx); ip.IsValid() {
		sender = ip.String()
	}
	for _, l := range []struct {
		key   string
		limit ratelimit.Limit
	}{
		{"showcase.contact:" + sender, limitContactSender},
		{"showcase.contact:" + row.ProjectID.String() + ":" + email, limitContactEmail},
		{"showcase.contact:" + row.ProjectID.String(), limitContactProject},
	} {
		if err := s.Limiter.Allow(ctx, l.key, l.limit, "showcase.too_many_requests"); err != nil {
			return apigen.ContactAccepted{}, err
		}
	}
	settings, err := s.Tenants.CurrentSettings(ctx)
	if err != nil {
		return apigen.ContactAccepted{}, err
	}
	status := ContactForwarded
	if settings.ContactModeration {
		status = ContactPending
	}
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		c, err := s.q(ctx).CreateContact(ctx, showcasedb.CreateContactParams{
			ProjectID: row.ProjectID, Name: name, Email: email, Organization: organization, Kind: string(body.Kind),
			Message: message, Locale: requestLocale(ctx, t), Status: status,
		})
		if err != nil {
			return err
		}
		if status == ContactForwarded {
			if err := s.forward(ctx, &c, &row); err != nil {
				return err
			}
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "showcase.contact_requested", TargetType: "project", TargetID: row.ProjectID.String(),
			Metadata: map[string]any{"contact_id": c.ID, "kind": c.Kind, "status": status},
		})
	})
	if err != nil {
		return apigen.ContactAccepted{}, fmt.Errorf("showcase: contact: %w", err)
	}
	return apigen.ContactAccepted{Status: apigen.ContactAcceptedStatus(status)}, nil
}

// forward sends the request to the owner and members of the team (inside
// the transaction of ctx).
func (s *Service) forward(ctx context.Context, c *showcasedb.ContactRequest, e *showcasedb.ShowcaseEntry) error {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return err
	}
	people, err := s.Projects.MemberIDs(ctx, c.ProjectID)
	if err != nil {
		return err
	}
	var team []uuid.UUID
	for id, role := range people {
		if role == project.RoleOwner || role == project.RoleMember {
			team = append(team, id)
		}
	}
	recipients, err := s.Users.Recipients(ctx, team)
	if err != nil {
		return err
	}
	sender := c.Name
	if c.Organization != nil {
		sender += " (" + *c.Organization + ")"
	}
	for _, r := range recipients {
		if err := s.Mailer.Send(ctx, r.Email, r.FullName, qmail.TemplateContactRequest, r.Locale, map[string]string{
			"name": r.FullName, "project": e.Title, "sender": sender, "kind": kindLabels[c.Kind][r.Locale],
			"message": c.Message, "email": c.Email, "link": t.Origin + "/app/projects/" + c.ProjectID.String() + "/showcase",
		}); err != nil {
			return err
		}
	}
	return nil
}

// contactPage reads the cursor and the size of a page of requests.
func contactPage(cursor *string, size *int) (*uuid.UUID, int, error) {
	limit := pagination.Limit(size)
	if cursor == nil {
		return nil, limit, nil
	}
	var before uuid.UUID
	if err := pagination.Decode(*cursor, &before); err != nil {
		return nil, limit, err
	}
	return &before, limit, nil
}

// contactList turns rows (one more than limit when another page follows)
// into a page.
func contactList(rows []showcasedb.ContactRequest, titles []string, limit int) apigen.ContactRequestList {
	out := apigen.ContactRequestList{Items: []apigen.ContactRequest{}}
	if len(rows) > limit {
		rows = rows[:limit]
		next := pagination.Encode(rows[len(rows)-1].ID)
		out.NextCursor = &next
	}
	for i := range rows {
		out.Items = append(out.Items, contactAPI(&rows[i], titles[i]))
	}
	return out
}

// Contacts lists the requests of the university for moderation, newest first.
func (s *Service) Contacts(ctx context.Context, status *apigen.ContactStatus, cursor *string, size *int) (apigen.ContactRequestList, error) {
	if _, err := authz.RequireRole(ctx, reviewRoles...); err != nil {
		return apigen.ContactRequestList{}, err
	}
	before, limit, err := contactPage(cursor, size)
	if err != nil {
		return apigen.ContactRequestList{}, err
	}
	var want *string
	if status != nil {
		v := string(*status)
		want = &v
	}
	var rows []showcasedb.ContactRequest
	var titles []string
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		found, err := s.q(ctx).ListContacts(ctx, showcasedb.ListContactsParams{
			Status: want, BeforeID: before, RowLimit: int32(limit + 1), //nolint:gosec // bounded by MaxLimit
		})
		for i := range found {
			r := &found[i]
			rows = append(rows, showcasedb.ContactRequest{
				ID: r.ID, TenantID: r.TenantID, ProjectID: r.ProjectID, Name: r.Name, Email: r.Email, Organization: r.Organization,
				Kind: r.Kind, Message: r.Message, Locale: r.Locale, Status: r.Status, ModeratedBy: r.ModeratedBy,
				ModeratedAt: r.ModeratedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			})
			titles = append(titles, r.ProjectTitle)
		}
		return err
	})
	if err != nil {
		return apigen.ContactRequestList{Items: []apigen.ContactRequest{}}, fmt.Errorf("showcase: contacts: %w", err)
	}
	return contactList(rows, titles, limit), nil
}

// ProjectContacts lists the requests to a project: forwarded ones for the
// team, all of them for innovation managers.
func (s *Service) ProjectContacts(ctx context.Context, projectID uuid.UUID, cursor *string, size *int) (apigen.ContactRequestList, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ContactRequestList{}, err
	}
	if !a.Team() && !a.CanModerate() {
		return apigen.ContactRequestList{}, errForbidden
	}
	before, limit, err := contactPage(cursor, size)
	if err != nil {
		return apigen.ContactRequestList{}, err
	}
	var rows []showcasedb.ContactRequest
	var titles []string
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		found, err := s.q(ctx).ListProjectContacts(ctx, showcasedb.ListProjectContactsParams{
			ProjectID: projectID, ForwardedOnly: !a.CanModerate(), BeforeID: before,
			RowLimit: int32(limit + 1), //nolint:gosec // bounded by MaxLimit
		})
		for i := range found {
			r := &found[i]
			rows = append(rows, showcasedb.ContactRequest{
				ID: r.ID, TenantID: r.TenantID, ProjectID: r.ProjectID, Name: r.Name, Email: r.Email, Organization: r.Organization,
				Kind: r.Kind, Message: r.Message, Locale: r.Locale, Status: r.Status, ModeratedBy: r.ModeratedBy,
				ModeratedAt: r.ModeratedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			})
			titles = append(titles, r.ProjectTitle)
		}
		return err
	})
	if err != nil {
		return apigen.ContactRequestList{Items: []apigen.ContactRequest{}}, fmt.Errorf("showcase: project contacts: %w", err)
	}
	return contactList(rows, titles, limit), nil
}

// Moderate forwards a pending request to the team or rejects it.
func (s *Service) Moderate(ctx context.Context, contactID uuid.UUID, forward bool) (apigen.ContactRequest, error) {
	p, err := authz.RequireRole(ctx, reviewRoles...)
	if err != nil {
		return apigen.ContactRequest{}, err
	}
	var out apigen.ContactRequest
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		c, err := q.LockContact(ctx, contactID)
		if db.IsNoRows(err) {
			return errContactNotFound
		}
		if err != nil {
			return err
		}
		if c.Status != ContactPending {
			return errAlreadyModerated
		}
		e, err := q.GetEntry(ctx, c.ProjectID)
		if err != nil {
			return err
		}
		// The team closed the form meanwhile: the request is not sent to them.
		if forward && !e.ContactEnabled {
			return errContactDisabled
		}
		status, action := ContactRejected, "showcase.contact_rejected"
		if forward {
			status, action = ContactForwarded, "showcase.contact_forwarded"
		}
		if c, err = q.ModerateContact(ctx, showcasedb.ModerateContactParams{ID: contactID, Status: status, ModeratedBy: &p.UserID}); err != nil {
			return err
		}
		if forward {
			if err := s.forward(ctx, &c, &e); err != nil {
				return err
			}
		}
		out = contactAPI(&c, e.Title)
		return s.Audit.Record(ctx, audit.Event{Action: action, TargetType: "contact_request", TargetID: contactID.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return out, err
		}
		return out, fmt.Errorf("showcase: moderate: %w", err)
	}
	return out, nil
}
