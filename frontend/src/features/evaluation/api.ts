import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import { useIdempotencyKey } from '@/shared/api/idempotency';
import type { components } from '@/shared/api/schema.gen';
import type { Locale } from '@/shared/i18n';

export type Evaluation = components['schemas']['Evaluation'];
export type EvaluationSummary = components['schemas']['EvaluationSummary'];
export type EvaluationBrief = components['schemas']['EvaluationBrief'];
export type EvaluationStatus = components['schemas']['EvaluationStatus'];
export type CriterionScore = components['schemas']['CriterionScore'];
export type RedFlag = components['schemas']['RedFlag'];
export type RedFlagSeverity = components['schemas']['RedFlagSeverity'];
export type Experiment = components['schemas']['Experiment'];
export type ReadinessLevel = components['schemas']['ReadinessLevel'];
export type ScoreOverride = components['schemas']['ScoreOverride'];
export type Rubric = components['schemas']['Rubric'];

/** An evaluation still being written by the model. */
export function isRunning(status: EvaluationStatus | undefined): boolean {
  return status === 'pending' || status === 'running';
}

/** Interval of polling a running evaluation. */
export const POLL_MS = 2_000;

export function evaluationsQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'evaluations'],
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/projects/{projectId}/evaluations', { params: { path: { projectId } } }),
        )
      ).items,
    // While one runs the history shows its status; refresh until it settles.
    refetchInterval: (query) =>
      query.state.data?.some((e) => isRunning(e.status)) ? POLL_MS : false,
  });
}

export function evaluationQuery(projectId: string, evaluationId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'evaluations', evaluationId],
    queryFn: () =>
      unwrap(
        api.GET('/projects/{projectId}/evaluations/{evaluationId}', {
          params: { path: { projectId, evaluationId } },
        }),
      ),
    refetchInterval: (query) => (isRunning(query.state.data?.status) ? POLL_MS : false),
  });
}

/** Address of the PDF report in a language. */
export function reportUrl(projectId: string, evaluationId: string, locale: Locale): string {
  return `/api/v1/projects/${projectId}/evaluations/${evaluationId}/report?locale=${locale}`;
}

function useInvalidateEvaluations(projectId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['projects', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['projects', 'list'] }),
    ]);
}

export function useRequestEvaluation(projectId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateEvaluations(projectId);
  const submission = useIdempotencyKey();
  return useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/projects/{projectId}/evaluations', {
          params: {
            path: { projectId },
            header: { 'Idempotency-Key': submission.for({ projectId }) },
          },
        }),
      ),
    onSettled: (_data, error) => {
      submission.settle(error);
    },
    onSuccess: (evaluation) => {
      queryClient.setQueryData(evaluationQuery(projectId, evaluation.id).queryKey, evaluation);
      void invalidate();
    },
  });
}

export function useOverrideScore(projectId: string, evaluationId: string) {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateEvaluations(projectId);
  return useMutation({
    mutationFn: (body: ScoreOverride) =>
      unwrap(
        api.POST('/projects/{projectId}/evaluations/{evaluationId}/overrides', {
          params: { path: { projectId, evaluationId } },
          body,
        }),
      ),
    onSuccess: (evaluation) => {
      queryClient.setQueryData(evaluationQuery(projectId, evaluationId).queryKey, evaluation);
      void invalidate();
    },
  });
}

export const rubricQuery = queryOptions({
  queryKey: ['admin', 'rubric'],
  queryFn: () => unwrapTagged(api.GET('/admin/rubric')),
});

export function useUpdateRubric() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      weights,
      etag,
    }: {
      weights: Record<string, number>;
      etag: string | undefined;
    }) =>
      unwrapTagged(
        api.PUT('/admin/rubric', {
          params: { header: etag ? { 'If-Match': etag } : {} },
          body: { weights },
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(rubricQuery.queryKey, saved);
    },
  });
}
