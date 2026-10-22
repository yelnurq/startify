package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai/aidb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// SubjectProjectStep marks conversations about a step of a project.
const SubjectProjectStep = "project_step"

// Topic identifies a conversation: what it is about, owned by the calling
// module (a step of a project).
type Topic struct {
	SubjectType string
	SubjectID   uuid.UUID
	Topic       string
}

// Message is a stored turn of a conversation.
type Message struct {
	ID            uuid.UUID
	Role          string
	AuthorID      *uuid.UUID
	Content       string
	Status        string
	Provider      string
	Model         string
	PromptVersion string
	Meta          json.RawMessage
	CreatedAt     time.Time
}

func messageOf(m *aidb.AiMessage) Message {
	out := Message{
		ID: m.ID, Role: m.Role, AuthorID: m.AuthorID, Content: m.Content, Status: m.Status, Meta: m.Meta, CreatedAt: m.CreatedAt,
	}
	if m.Provider != nil {
		out.Provider = *m.Provider
	}
	if m.Model != nil {
		out.Model = *m.Model
	}
	if m.PromptVersion != nil {
		out.PromptVersion = *m.PromptVersion
	}
	return out
}

func orNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Messages returns up to limit messages of a conversation before the given
// one (all when nil), newest first. A conversation that never started is empty.
func (g *Gateway) Messages(ctx context.Context, t Topic, before *uuid.UUID, limit int) ([]Message, error) {
	out := []Message{}
	err := g.db.ReadTx(ctx, func(ctx context.Context) error {
		q := aidb.New(g.db.Conn(ctx))
		c, err := q.FindConversation(ctx, aidb.FindConversationParams{SubjectType: t.SubjectType, SubjectID: t.SubjectID, Topic: t.Topic})
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		rows, err := q.ListMessages(ctx, aidb.ListMessagesParams{ConversationID: c.ID, BeforeID: before, RowLimit: int32(limit)}) //nolint:gosec // small page size
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, messageOf(&rows[i]))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ai: messages: %w", err)
	}
	return out, nil
}

// AddMessage appends a message to a conversation, starting it when needed.
// It joins the transaction of ctx when there is one.
func (g *Gateway) AddMessage(ctx context.Context, t Topic, m Message) (Message, error) {
	if m.Meta == nil {
		m.Meta = json.RawMessage("{}")
	}
	if m.Status == "" {
		m.Status = "ok"
	}
	var saved Message
	err := g.db.Tx(ctx, func(ctx context.Context) error {
		q := aidb.New(g.db.Conn(ctx))
		c, err := q.EnsureConversation(ctx, aidb.EnsureConversationParams{SubjectType: t.SubjectType, SubjectID: t.SubjectID, Topic: t.Topic})
		if err != nil {
			return err
		}
		row, err := q.InsertMessage(ctx, aidb.InsertMessageParams{
			ConversationID: c.ID, Role: m.Role, AuthorID: m.AuthorID, Content: m.Content, Status: m.Status,
			Provider: orNil(m.Provider), Model: orNil(m.Model), PromptVersion: orNil(m.PromptVersion), Meta: m.Meta,
		})
		if err != nil {
			return err
		}
		saved = messageOf(&row)
		return nil
	})
	if err != nil {
		return saved, fmt.Errorf("ai: add message: %w", err)
	}
	return saved, nil
}
