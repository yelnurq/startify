package incubator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/prompts"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
)

// Mentor conversation limits.
const (
	historyTurns     = 12
	knowledgeSources = 3
	mentorMaxTokens  = 1500
)

var errMentorForbidden = apperr.Forbidden("incubator.mentor_forbidden", "only the team talks to the mentor")

func topic(projectID uuid.UUID, step string) ai.Topic {
	return ai.Topic{SubjectType: ai.SubjectProjectStep, SubjectID: projectID, Topic: step}
}

// messageMeta is stored with an assistant message.
type messageMeta struct {
	Sources []apigen.MentorSource `json:"sources,omitempty"`
	Mode    string                `json:"mode,omitempty"`
}

func (s *Service) messagesAPI(ctx context.Context, msgs []ai.Message) ([]apigen.MentorMessage, error) {
	var people []uuid.UUID
	for i := range msgs {
		if msgs[i].AuthorID != nil {
			people = append(people, *msgs[i].AuthorID)
		}
	}
	briefs, err := s.Users.Briefs(ctx, people)
	if err != nil {
		return nil, err
	}
	out := make([]apigen.MentorMessage, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		var meta messageMeta
		_ = json.Unmarshal(m.Meta, &meta)
		mm := apigen.MentorMessage{
			Id: m.ID, Role: apigen.MentorMessageRole(m.Role), Content: m.Content, Status: apigen.MentorMessageStatus(m.Status),
			Sources: meta.Sources, CreatedAt: m.CreatedAt,
		}
		if mm.Sources == nil {
			mm.Sources = []apigen.MentorSource{}
		}
		if m.AuthorID != nil {
			b := briefs[*m.AuthorID]
			mm.Author = &b
		}
		out[i] = mm
	}
	return out, nil
}

// modes lists the mentor modes the university allows, the default first: a
// co-author university lets the team switch to coaching.
func modes(settings ai.Settings) []apigen.MentorMode {
	if settings.MentorMode == "coauthor" {
		return []apigen.MentorMode{apigen.MentorModeCoauthor, apigen.MentorModeCoach}
	}
	return []apigen.MentorMode{apigen.MentorModeCoach}
}

type messageCursor struct {
	ID uuid.UUID `json:"i"`
}

// Conversation returns the mentor conversation about a step, oldest first.
func (s *Service) Conversation(ctx context.Context, projectID uuid.UUID, key string, params apigen.GetMentorConversationParams) (apigen.MentorConversation, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.MentorConversation{}, err
	}
	if _, err := s.step(key); err != nil {
		return apigen.MentorConversation{}, err
	}
	limit := 30
	if params.Limit != nil {
		limit = min(max(*params.Limit, 1), 100)
	}
	var before *uuid.UUID
	if params.Cursor != nil {
		var c messageCursor
		if err := pagination.Decode(*params.Cursor, &c); err != nil {
			return apigen.MentorConversation{}, err
		}
		before = &c.ID
	}
	msgs, err := s.Gateway.Messages(ctx, topic(projectID, key), before, limit+1)
	if err != nil {
		return apigen.MentorConversation{}, err
	}
	settings, err := s.Gateway.Settings(ctx)
	if err != nil {
		return apigen.MentorConversation{}, err
	}
	out := apigen.MentorConversation{
		Modes: modes(settings), External: s.Gateway.External(settings, llm.FeatureMentor), CanAsk: a.CanEdit(),
	}
	if len(msgs) > limit {
		msgs = msgs[:limit]
		next := pagination.Encode(messageCursor{ID: msgs[len(msgs)-1].ID})
		out.NextCursor = &next
	}
	slices.Reverse(msgs)
	if out.Items, err = s.messagesAPI(ctx, msgs); err != nil {
		return out, err
	}
	return out, nil
}

// projectContext is the JSON the mentor and the materials see: the card,
// the step at hand and every answered field of the project.
func (s *Service) projectContext(ctx context.Context, p project.Access, step string) (string, Values, error) {
	var rows []incubatordb.ProjectStepDatum
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListSteps(ctx, p.Project.ID)
		return err
	})
	if err != nil {
		return "", nil, fmt.Errorf("incubator: context: %w", err)
	}
	return contextJSON(p.Project.Title, p.Project.Tagline, p.Project.Industry, step, rows)
}

func contextJSON(title, tagline, industry, step string, rows []incubatordb.ProjectStepDatum) (string, Values, error) {
	d := map[string]any{"project": map[string]any{"title": title, "tagline": tagline, "industry": industry}}
	steps := map[string]Values{}
	current := Values{}
	for i := range rows {
		r := &rows[i]
		v := decodeValues(r.Data)
		if len(v) > 0 {
			steps[r.StepKey] = v
		}
		if r.StepKey == step {
			current = v
		}
	}
	d["steps"] = steps
	if step != "" {
		d["step"] = map[string]any{"key": step, "values": current}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return "", nil, err
	}
	return string(raw), current, nil
}

// mentorCall is what the stream needs to store the answer.
type mentorCall struct {
	topic   ai.Topic
	sources []apigen.MentorSource
	mode    string
	user    uuid.UUID
	request *http.Request
}

// Ask saves the team's question and starts the mentor's streamed answer.
func (s *Service) Ask(ctx context.Context, projectID uuid.UUID, key string, body apigen.MentorMessageCreate) (*MentorStream, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if a.Archived() {
		return nil, apperr.Conflict("project.archived", "the project is archived")
	}
	if !a.Team() {
		return nil, errMentorForbidden
	}
	st, err := s.step(key)
	if err != nil {
		return nil, err
	}
	var v validate.Errors
	question := v.Paragraph("content", body.Content, 1, 4000)
	if err := v.Err(); err != nil {
		return nil, err
	}
	if err := s.Limiter.Allow(ctx, "mentor:"+a.Principal.UserID.String(), limitMentor, "ai.too_many_requests"); err != nil {
		return nil, err
	}
	settings, err := s.Gateway.Settings(ctx)
	if err != nil {
		return nil, err
	}
	allowed := modes(settings)
	mode := string(allowed[0])
	if body.Mode != nil && slices.Contains(allowed, *body.Mode) {
		mode = string(*body.Mode)
	}
	lang := locale(ctx)
	data, _, err := s.projectContext(ctx, a, key)
	if err != nil {
		return nil, err
	}

	var sources []apigen.MentorSource
	var excerpts []prompts.Source
	if s.Knowledge != nil {
		hits, err := s.Knowledge.Search(ctx, question+" "+st.Title.Best(lang), knowledgeSources)
		if err == nil {
			for i := range hits {
				h := &hits[i]
				excerpts = append(excerpts, prompts.Source{Title: h.Title, Text: h.Text})
				if !slices.ContainsFunc(sources, func(x apigen.MentorSource) bool { return x.DocumentId == h.DocumentID }) {
					sources = append(sources, apigen.MentorSource{DocumentId: h.DocumentID, Title: h.Title})
				}
			}
		}
	}
	criteria := make([]string, len(st.Criteria))
	for i, c := range st.Criteria {
		criteria[i] = c.Text.Best("en")
	}
	var fields []prompts.FieldRef
	for i := range st.Fields {
		f := &st.Fields[i]
		if f.Type == TypeText || f.Type == TypeTextarea || f.Type == TypeList {
			fields = append(fields, prompts.FieldRef{Key: f.Key, Label: f.Label.Best("en")})
		}
	}
	system, version, err := prompts.Render("mentor", prompts.Mentor{
		StepNumber: st.Number(), StepName: st.Title.Best("en"), StepGoal: st.Goal.Best("en"), Criteria: criteria,
		Fields: fields, ProjectTitle: a.Project.Title, Mode: mode, Locale: lang, ProjectData: data, Knowledge: excerpts,
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}

	t := topic(projectID, key)
	history, err := s.Gateway.Messages(ctx, t, nil, historyTurns)
	if err != nil {
		return nil, err
	}
	slices.Reverse(history)
	var messages []llm.Message
	for i := range history {
		m := &history[i]
		if m.Status == "error" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		role := llm.RoleUser
		if m.Role == "assistant" {
			role = llm.RoleAssistant
		}
		messages = append(messages, llm.Message{Role: role, Content: m.Content})
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: question})

	// Start the model first: a refused request (limits, no provider) leaves
	// no unanswered question behind.
	streamCtx, cancel := context.WithCancel(ctx)
	events, err := s.Gateway.Stream(streamCtx, ai.Call{
		Feature: llm.FeatureMentor, PromptVersion: version, System: system, Messages: messages, Locale: lang,
		MaxTokens: mentorMaxTokens, Effort: llm.EffortMedium,
	})
	if err != nil {
		cancel()
		return nil, err
	}
	stop := func() {
		cancel()
		for range events {
		}
	}
	user := a.Principal.UserID
	saved, err := s.Gateway.AddMessage(ctx, t, ai.Message{Role: "user", AuthorID: &user, Content: question})
	if err != nil {
		stop()
		return nil, err
	}
	asked, err := s.messagesAPI(ctx, []ai.Message{saved})
	if err != nil {
		stop()
		return nil, err
	}
	r, ok := httpx.RequestFrom(ctx)
	if !ok {
		r = &http.Request{}
	}
	return &MentorStream{
		svc: s, ctx: ctx, cancel: cancel, question: asked[0], events: events,
		call: mentorCall{topic: t, sources: sources, mode: mode, user: user, request: r},
	}, nil
}

// MentorStream relays the mentor's answer as server-sent events and stores
// it when it ends, also when the client goes away.
type MentorStream struct {
	svc *Service
	// ctx is the request's: the answer is stored in its university after
	// the handler has returned the stream.
	ctx      context.Context //nolint:containedctx // see above
	cancel   context.CancelFunc
	question apigen.MentorMessage
	events   <-chan ai.Event
	call     mentorCall
}

// VisitSendMentorMessageResponse implements apigen.SendMentorMessageResponseObject.
func (m *MentorStream) VisitSendMentorMessageResponse(w http.ResponseWriter) error {
	defer m.cancel()
	sse := httpx.NewSSE(w)
	gone := sse.Event("message", m.question) != nil
	var text strings.Builder
	var result *ai.Result
	var failure error
	for ev := range m.events {
		switch {
		case ev.Err != nil:
			failure = ev.Err
		case ev.Result != nil:
			result = ev.Result
		default:
			text.WriteString(ev.Delta)
			// A failed write means the client left: stop the model.
			if !gone && sse.Event("delta", map[string]string{"text": ev.Delta}) != nil {
				gone = true
				m.cancel()
			}
		}
	}
	// The answer is kept even when the request was cancelled.
	ctx := context.WithoutCancel(m.ctx)
	msg := ai.Message{Role: "assistant", Content: text.String()}
	meta, _ := json.Marshal(messageMeta{Sources: m.call.sources, Mode: m.call.mode})
	msg.Meta = meta
	switch {
	case result != nil:
		msg.Content, msg.Provider, msg.Model, msg.PromptVersion = result.Text, result.Provider, result.Model, result.PromptVersion
	case failure != nil && !gone && m.ctx.Err() == nil:
		msg.Status = "error"
	default:
		msg.Status = "cancelled"
	}
	saved, err := m.svc.Gateway.AddMessage(ctx, m.call.topic, msg)
	if gone {
		return nil // the client is gone; the answer is stored as cancelled
	}
	if err != nil {
		_ = sse.Event("error", httpx.ProblemFrom(m.call.request, err))
		return nil
	}
	if msg.Status == "error" {
		_ = sse.Event("error", httpx.ProblemFrom(m.call.request, failure))
		return nil
	}
	out, err := m.svc.messagesAPI(ctx, []ai.Message{saved})
	if err != nil {
		_ = sse.Event("error", httpx.ProblemFrom(m.call.request, err))
		return nil
	}
	_ = sse.Event("done", out[0])
	return nil
}
