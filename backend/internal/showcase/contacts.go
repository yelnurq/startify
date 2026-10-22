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

	// A partner writes a few letters an hour; a project receives at most a
	// few dozen a day.
	limitContactSender  = ratelimit.PerHour(5)
	limitContactProject = ratelimit.Limit{Rate: 20, Burst: 20, Period: 24 * time.Hour}

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
	email := strings.ToLower(strings.TrimSpace(string(body.Email)))
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
	if err := s.Limiter.Allow(ctx, "showcase.contact:"+sender, limitContactSender, "showcase.too_many_requests"); err != nil {
		return apigen.ContactAccepted{}, err
	}
	if err := s.Limiter.Allow(ctx, "showcase.contact:"+row.ProjectID.String(), limitContactProject, "showcase.too_many_requests"); err != nil {
		return apigen.ContactAccepted{}, err
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

// Contacts lists the requests of the university for moderation, newest first.
func (s *Service) Contacts(ctx context.Context, status *apigen.ContactStatus) (apigen.ContactRequestList, error) {
	if _, err := authz.RequireRole(ctx, reviewRoles...); err != nil {
		return apigen.ContactRequestList{}, err
	}
	var want *string
	if status != nil {
		v := string(*status)
		want = &v
	}
	out := apigen.ContactRequestList{Items: []apigen.ContactRequest{}}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListContacts(ctx, want)
		for i := range rows {
			r := &rows[i]
			c := showcasedb.ContactRequest{
				ID: r.ID, TenantID: r.TenantID, ProjectID: r.ProjectID, Name: r.Name, Email: r.Email, Organization: r.Organization,
				Kind: r.Kind, Message: r.Message, Locale: r.Locale, Status: r.Status, ModeratedBy: r.ModeratedBy,
				ModeratedAt: r.ModeratedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			}
			out.Items = append(out.Items, contactAPI(&c, r.ProjectTitle))
		}
		return err
	})
	if err != nil {
		return out, fmt.Errorf("showcase: contacts: %w", err)
	}
	return out, nil
}

// ProjectContacts lists the requests to a project: forwarded ones for the
// team, all of them for innovation managers.
func (s *Service) ProjectContacts(ctx context.Context, projectID uuid.UUID) (apigen.ContactRequestList, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ContactRequestList{}, err
	}
	if !a.Team() && !a.CanModerate() {
		return apigen.ContactRequestList{}, errForbidden
	}
	out := apigen.ContactRequestList{Items: []apigen.ContactRequest{}}
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListProjectContacts(ctx, showcasedb.ListProjectContactsParams{
			ProjectID: projectID, ForwardedOnly: !a.CanModerate(),
		})
		for i := range rows {
			r := &rows[i]
			c := showcasedb.ContactRequest{
				ID: r.ID, TenantID: r.TenantID, ProjectID: r.ProjectID, Name: r.Name, Email: r.Email, Organization: r.Organization,
				Kind: r.Kind, Message: r.Message, Locale: r.Locale, Status: r.Status, ModeratedBy: r.ModeratedBy,
				ModeratedAt: r.ModeratedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			}
			out.Items = append(out.Items, contactAPI(&c, r.ProjectTitle))
		}
		return err
	})
	if err != nil {
		return out, fmt.Errorf("showcase: project contacts: %w", err)
	}
	return out, nil
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
