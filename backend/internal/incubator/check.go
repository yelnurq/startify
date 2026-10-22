package incubator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/prompts"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// checkSchema constrains the AI review of a step.
var checkSchema = llm.Schema{Name: "step_check", JSON: map[string]any{
	"type":     "object",
	"required": []any{"ready", "summary", "items"},
	"properties": map[string]any{
		"ready":   map[string]any{"type": "boolean"},
		"summary": text(600),
		"items": map[string]any{
			"type": "array", "minItems": 1, "maxItems": 8,
			"items": map[string]any{
				"type":     "object",
				"required": []any{"criterion", "status", "advice"},
				"properties": map[string]any{
					"criterion": text(300),
					"status":    map[string]any{"type": "string", "enum": []any{"ok", "weak", "missing"}},
					"advice":    text(600),
				},
			},
		},
	},
}}

// Check asks the AI to review a step against its criteria and keeps the
// review with the step.
func (s *Service) Check(ctx context.Context, projectID uuid.UUID, key string) (apigen.Step, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Step{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Step{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	if err := s.Limiter.Allow(ctx, "check:"+a.Principal.UserID.String(), limitCheck, "ai.too_many_requests"); err != nil {
		return apigen.Step{}, err
	}
	var row incubatordb.ProjectStepDatum
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		row, err = s.q(ctx).GetStep(ctx, incubatordb.GetStepParams{ProjectID: projectID, StepKey: key})
		return err
	})
	if db.IsNoRows(err) {
		return apigen.Step{}, apperr.New(apperr.KindValidation, "incubator.step_empty", "fill in the step before the review")
	}
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: check: %w", err)
	}
	lang := locale(ctx)
	criteria := make([]string, len(st.Criteria))
	for i, c := range st.Criteria {
		criteria[i] = c.Text.Best("en")
	}
	answers, err := json.Marshal(map[string]any{"step": key, "values": labelled(st, decodeValues(row.Data))})
	if err != nil {
		return apigen.Step{}, err
	}
	system, version, err := prompts.Render("step_check", prompts.StepCheck{
		StepNumber: st.Number(), StepName: st.Title.Best("en"), StepGoal: st.Goal.Best("en"),
		Criteria: criteria, Locale: lang, Answers: string(answers),
	})
	if err != nil {
		return apigen.Step{}, apperr.Internal(err)
	}
	res, err := s.Gateway.Complete(ctx, ai.Call{
		Feature: llm.FeatureMentor, PromptVersion: version, System: system, Locale: lang, Schema: &checkSchema,
		MaxTokens: 2000, Effort: llm.EffortLow,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Review the step."}},
	})
	if err != nil {
		return apigen.Step{}, err
	}
	var check apigen.StepCheck
	if err := json.Unmarshal([]byte(res.Text), &check); err != nil {
		raw, _ := json.Marshal(res.JSON)
		if err := json.Unmarshal(raw, &check); err != nil {
			return apigen.Step{}, apperr.Internal(fmt.Errorf("incubator: check result: %w", err))
		}
	}
	check.Model, check.PromptVersion, check.CheckedAt, check.Version = res.Model, version, s.now(), int(row.Version)
	stored, err := json.Marshal(check)
	if err != nil {
		return apigen.Step{}, err
	}
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		var err error
		row, err = s.q(ctx).SetStepCheck(ctx, incubatordb.SetStepCheckParams{ProjectID: projectID, StepKey: key, AiCheck: stored})
		return err
	})
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: check: %w", err)
	}
	return s.stepAPI(ctx, st, &row)
}

// labelled keeps only the answers to the template's fields.
func labelled(st *Step, values Values) Values {
	out := Values{}
	for i := range st.Fields {
		if v, ok := values[st.Fields[i].Key]; ok {
			out[st.Fields[i].Key] = v
		}
	}
	return out
}
