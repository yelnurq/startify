package incubator

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
)

var errCommentNotFound = apperr.NotFound("incubator.comment_not_found", "the comment does not exist")

type commentCursor struct {
	ID uuid.UUID `json:"i"`
}

func (s *Service) commentsAPI(ctx context.Context, rows []incubatordb.ProjectComment, viewer uuid.UUID) ([]apigen.Comment, error) {
	var people []uuid.UUID
	for i := range rows {
		if rows[i].AuthorID != nil {
			people = append(people, *rows[i].AuthorID)
		}
		people = append(people, rows[i].Mentions...)
	}
	briefs, err := s.Users.Briefs(ctx, people)
	if err != nil {
		return nil, err
	}
	out := make([]apigen.Comment, len(rows))
	for i := range rows {
		r := &rows[i]
		c := apigen.Comment{
			Id: r.ID, StepKey: r.StepKey, FieldKey: r.FieldKey, ParentId: r.ParentID, Body: r.Body,
			Mentions: []apigen.MemberBrief{}, Resolved: r.ResolvedAt != nil, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Editable: r.AuthorID != nil && *r.AuthorID == viewer,
		}
		if r.AuthorID != nil {
			b := briefs[*r.AuthorID]
			c.Author = &b
		}
		for _, m := range r.Mentions {
			if b, ok := briefs[m]; ok {
				c.Mentions = append(c.Mentions, b)
			}
		}
		out[i] = c
	}
	return out, nil
}

// Comments lists comments of a project, optionally of one step.
func (s *Service) Comments(ctx context.Context, projectID uuid.UUID, params apigen.ListProjectCommentsParams) (apigen.CommentPage, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.CommentPage{}, err
	}
	limit, err := pagination.LimitOr(params.Limit, 50, pagination.MaxLimit)
	if err != nil {
		return apigen.CommentPage{}, err
	}
	args := incubatordb.ListCommentsParams{ProjectID: projectID, StepKey: params.StepKey, RowLimit: int32(limit + 1)} //nolint:gosec // bounded by MaxLimit
	if params.Cursor != nil {
		var c commentCursor
		if err := pagination.Decode(*params.Cursor, &c); err != nil {
			return apigen.CommentPage{}, err
		}
		args.AfterID = &c.ID
	}
	var rows []incubatordb.ProjectComment
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListComments(ctx, args)
		return err
	})
	if err != nil {
		return apigen.CommentPage{}, fmt.Errorf("incubator: comments: %w", err)
	}
	page := apigen.CommentPage{}
	if len(rows) > limit {
		rows = rows[:limit]
		next := pagination.Encode(commentCursor{ID: rows[len(rows)-1].ID})
		page.NextCursor = &next
	}
	if page.Items, err = s.commentsAPI(ctx, rows, a.Principal.UserID); err != nil {
		return page, err
	}
	return page, nil
}

var errArchived = apperr.Conflict("project.archived", "the project is archived")

// limitComments bounds the comments a member writes: a busy review is a few
// dozen an hour.
var limitComments = ratelimit.Limit{Rate: 120, Burst: 30, Period: time.Hour}

// canComment: the team, the supervisor and innovation managers, while the
// project is open.
func canComment(a project.Access) error {
	switch {
	case a.Archived():
		return errArchived
	case !a.CanComment():
		return errForbidden
	}
	return nil
}

// AddComment comments on a step or a field and mentions people of the project.
func (s *Service) AddComment(ctx context.Context, projectID uuid.UUID, body apigen.CommentCreate) (apigen.Comment, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Comment{}, err
	}
	if err := canComment(a); err != nil {
		return apigen.Comment{}, err
	}
	var v validate.Errors
	st, ok := s.tmpl.Step(body.StepKey)
	if !ok {
		v.Add("step_key", validate.CodeInvalid, nil)
	}
	if ok && body.FieldKey != nil && st.Field(*body.FieldKey) == nil {
		v.Add("field_key", validate.CodeInvalid, nil)
	}
	text := v.Paragraph("body", body.Body, 1, 2000)
	mentions := []uuid.UUID{}
	if body.Mentions != nil {
		people, err := s.Projects.MemberIDs(ctx, projectID)
		if err != nil {
			return apigen.Comment{}, err
		}
		for _, m := range *body.Mentions {
			if _, ok := people[m]; !ok {
				v.Add("mentions", validate.CodeInvalid, nil)
				break
			}
			if !slices.Contains(mentions, m) {
				mentions = append(mentions, m)
			}
		}
		if len(mentions) > 10 {
			v.Add("mentions", validate.CodeMaxItems, map[string]any{"count": 10})
		}
	}
	if err := v.Err(); err != nil {
		return apigen.Comment{}, err
	}
	author := a.Principal.UserID
	if err := s.Limiter.Allow(ctx, "comment:"+author.String(), limitComments, "rate_limited"); err != nil {
		return apigen.Comment{}, err
	}
	var row incubatordb.ProjectComment
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if body.ParentId != nil {
			parent, err := q.GetComment(ctx, incubatordb.GetCommentParams{ID: *body.ParentId, ProjectID: projectID})
			if db.IsNoRows(err) || (err == nil && (parent.ParentID != nil || parent.StepKey != body.StepKey)) {
				return apperr.NotFound("incubator.parent_not_found", "the thread does not exist")
			}
			if err != nil {
				return err
			}
		}
		var err error
		row, err = q.CreateComment(ctx, incubatordb.CreateCommentParams{
			ProjectID: projectID, StepKey: body.StepKey, FieldKey: body.FieldKey, ParentID: body.ParentId,
			AuthorID: &author, Body: text, Mentions: mentions,
		})
		if err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "incubator.comment_added", TargetType: "project", TargetID: projectID.String(),
			Metadata: map[string]any{"step": body.StepKey, "comment_id": row.ID},
		})
	})
	if err != nil {
		return apigen.Comment{}, fmt.Errorf("incubator: comment: %w", err)
	}
	out, err := s.commentsAPI(ctx, []incubatordb.ProjectComment{row}, author)
	if err != nil {
		return apigen.Comment{}, err
	}
	return out[0], nil
}

// UpdateComment edits the author's text or resolves a thread.
func (s *Service) UpdateComment(ctx context.Context, projectID, commentID uuid.UUID, body apigen.CommentUpdate) (apigen.Comment, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Comment{}, err
	}
	if err := canComment(a); err != nil {
		return apigen.Comment{}, err
	}
	user := a.Principal.UserID
	var row incubatordb.ProjectComment
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		c, err := q.GetComment(ctx, incubatordb.GetCommentParams{ID: commentID, ProjectID: projectID})
		if db.IsNoRows(err) {
			return errCommentNotFound
		}
		if err != nil {
			return err
		}
		args := incubatordb.UpdateCommentParams{ID: commentID, Resolved: body.Resolved, UserID: &user}
		if body.Body != nil {
			if c.AuthorID == nil || *c.AuthorID != user {
				return errForbidden
			}
			var v validate.Errors
			text := v.Paragraph("body", *body.Body, 1, 2000)
			if err := v.Err(); err != nil {
				return err
			}
			args.Body = &text
		}
		if row, err = q.UpdateComment(ctx, args); err != nil {
			return err
		}
		for _, change := range []struct {
			happened bool
			action   string
		}{
			{body.Body != nil && *args.Body != c.Body, "incubator.comment_edited"},
			{row.ResolvedAt != nil && c.ResolvedAt == nil, "incubator.comment_resolved"},
			{row.ResolvedAt == nil && c.ResolvedAt != nil, "incubator.comment_reopened"},
		} {
			if !change.happened {
				continue
			}
			if err := s.Audit.Record(ctx, audit.Event{
				Action: change.action, TargetType: "project", TargetID: projectID.String(),
				Metadata: map[string]any{"step": c.StepKey, "comment_id": commentID},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return apigen.Comment{}, fmt.Errorf("incubator: update comment: %w", err)
	}
	out, err := s.commentsAPI(ctx, []incubatordb.ProjectComment{row}, user)
	if err != nil {
		return apigen.Comment{}, err
	}
	return out[0], nil
}

// DeleteComment removes a comment with its replies (the author or a
// moderator) while the project is open.
func (s *Service) DeleteComment(ctx context.Context, projectID, commentID uuid.UUID) error {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return err
	}
	if a.Archived() {
		return errArchived
	}
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		c, err := q.GetComment(ctx, incubatordb.GetCommentParams{ID: commentID, ProjectID: projectID})
		if db.IsNoRows(err) {
			return errCommentNotFound
		}
		if err != nil {
			return err
		}
		own := c.AuthorID != nil && *c.AuthorID == a.Principal.UserID
		if !own && !a.CanModerate() {
			return errForbidden
		}
		if err := q.DeleteComment(ctx, commentID); err != nil {
			return err
		}
		// A moderator removing someone's thread is recorded with its author.
		return s.Audit.Record(ctx, audit.Event{
			Action: "incubator.comment_deleted", TargetType: "project", TargetID: projectID.String(),
			Metadata: map[string]any{
				"step": c.StepKey, "comment_id": commentID, "author_id": c.AuthorID, "thread": c.ParentID == nil, "moderated": !own,
			},
		})
	})
	if err != nil {
		return fmt.Errorf("incubator: delete comment: %w", err)
	}
	return nil
}

// Presence: members seen within presenceTTL are "here".
const presenceTTL = 45 * time.Second

func presenceKey(projectID uuid.UUID) string { return "presence:" + projectID.String() }

// Presence records the member in the project and returns who else is here
// with the current version of every step.
func (s *Service) Presence(ctx context.Context, projectID uuid.UUID, step *string) (apigen.Presence, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Presence{}, err
	}
	where := ""
	if step != nil {
		if _, ok := s.tmpl.Step(*step); ok {
			where = *step
		}
	}
	now := s.now()
	key := presenceKey(projectID)
	member := a.Principal.UserID.String()
	// One hash field per member: "<unix seconds>|<step>".
	pipe := s.Cache.TxPipeline()
	pipe.HSet(ctx, key, member, strconv.FormatInt(now.Unix(), 10)+"|"+where)
	pipe.Expire(ctx, key, 2*presenceTTL)
	all := pipe.HGetAll(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		return apigen.Presence{}, apperr.Internal(fmt.Errorf("incubator: presence: %w", err))
	}
	out := apigen.Presence{Viewers: []apigen.Viewer{}, Versions: map[string]int{}}
	type seen struct {
		id   uuid.UUID
		at   time.Time
		step string
	}
	var here []seen
	var stale []string
	for field, val := range all.Val() {
		id, err := uuid.Parse(field)
		ts, where, ok := strings.Cut(val, "|")
		sec, perr := strconv.ParseInt(ts, 10, 64)
		if err != nil || !ok || perr != nil || now.Sub(time.Unix(sec, 0)) > presenceTTL {
			stale = append(stale, field)
			continue
		}
		here = append(here, seen{id: id, at: time.Unix(sec, 0), step: where})
	}
	if len(stale) > 0 {
		if err := s.Cache.HDel(ctx, key, stale...).Err(); err != nil && !errors.Is(err, redis.Nil) {
			return out, apperr.Internal(fmt.Errorf("incubator: presence: %w", err))
		}
	}
	sort.Slice(here, func(i, j int) bool { return here[i].at.After(here[j].at) })
	ids := make([]uuid.UUID, len(here))
	for i, h := range here {
		ids[i] = h.id
	}
	briefs, err := s.Users.Briefs(ctx, ids)
	if err != nil {
		return out, err
	}
	for _, h := range here {
		v := apigen.Viewer{User: briefs[h.id], SeenAt: h.at}
		if h.step != "" {
			st := h.step
			v.StepKey = &st
		}
		out.Viewers = append(out.Viewers, v)
	}
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListSteps(ctx, projectID)
		for i := range rows {
			out.Versions[rows[i].StepKey] = int(rows[i].Version)
		}
		return err
	})
	if err != nil {
		return out, fmt.Errorf("incubator: presence: %w", err)
	}
	return out, nil
}

// Activity merges recent edits, comments, completed steps and new members.
func (s *Service) Activity(ctx context.Context, projectID uuid.UUID, limit int) (apigen.ActivityList, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.ActivityList{}, err
	}
	limit = min(max(limit, 1), 100)
	n := int32(limit)
	var items []apigen.Activity
	var actors []uuid.UUID
	add := func(kind apigen.ActivityKind, actor *uuid.UUID, step, field *string, at time.Time) {
		items = append(items, apigen.Activity{Kind: kind, StepKey: step, FieldKey: field, At: at})
		if actor != nil {
			actors = append(actors, *actor)
			items[len(items)-1].Actor = &apigen.MemberBrief{Id: *actor}
		}
	}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		edits, err := q.RecentRevisions(ctx, incubatordb.RecentRevisionsParams{ProjectID: projectID, RowLimit: n})
		if err != nil {
			return err
		}
		for _, e := range edits {
			add(apigen.ActivityKindEdit, e.AuthorID, &e.StepKey, &e.FieldKey, e.UpdatedAt)
		}
		comments, err := q.RecentComments(ctx, incubatordb.RecentCommentsParams{ProjectID: projectID, RowLimit: n})
		if err != nil {
			return err
		}
		for _, c := range comments {
			add(apigen.ActivityKindComment, c.AuthorID, &c.StepKey, c.FieldKey, c.CreatedAt)
		}
		completions, err := q.RecentCompletions(ctx, incubatordb.RecentCompletionsParams{ProjectID: projectID, RowLimit: n})
		if err != nil {
			return err
		}
		for _, c := range completions {
			if c.CompletedAt == nil {
				continue
			}
			if c.Status == StatusApproved && c.ApprovedBy != nil {
				add(apigen.ActivityKindStepApproved, c.ApprovedBy, &c.StepKey, nil, *c.CompletedAt)
			} else {
				add(apigen.ActivityKindStepCompleted, c.CompletedBy, &c.StepKey, nil, *c.CompletedAt)
			}
		}
		return nil
	})
	if err != nil {
		return apigen.ActivityList{}, fmt.Errorf("incubator: activity: %w", err)
	}
	members, err := s.Projects.Members(ctx, projectID)
	if err != nil {
		return apigen.ActivityList{}, err
	}
	for i := range members {
		id := members[i].User.Id
		add(apigen.ActivityKindMemberJoined, &id, nil, nil, members[i].JoinedAt)
	}
	briefs, err := s.Users.Briefs(ctx, actors)
	if err != nil {
		return apigen.ActivityList{}, err
	}
	for i := range items {
		if items[i].Actor != nil {
			b := briefs[items[i].Actor.Id]
			items[i].Actor = &b
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].At.After(items[j].At) })
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []apigen.Activity{}
	}
	return apigen.ActivityList{Items: items}, nil
}
