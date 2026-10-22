import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';

import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type IncubatorTemplate = components['schemas']['IncubatorTemplate'];
export type TemplateStage = components['schemas']['TemplateStage'];
export type TemplateStep = components['schemas']['TemplateStep'];
export type Step = components['schemas']['Step'];
export type StepSummary = components['schemas']['StepSummary'];
export type StepList = components['schemas']['StepList'];
export type StepStatus = components['schemas']['StepStatus'];
export type StepCheck = components['schemas']['StepCheck'];
export type StepGap = components['schemas']['StepGap'];
export type Revision = components['schemas']['Revision'];
export type Comment = components['schemas']['Comment'];
export type CommentCreate = components['schemas']['CommentCreate'];
export type MentorMessage = components['schemas']['MentorMessage'];
export type MentorConversation = components['schemas']['MentorConversation'];
export type MentorMode = components['schemas']['MentorMode'];
export type Material = components['schemas']['Material'];
export type MaterialKind = components['schemas']['MaterialKind'];
export type MaterialFormat = components['schemas']['MaterialFormat'];

/** Text limits checked again by the server. */
export const INCUBATOR_LIMITS = {
  comment: 2000,
  mentions: 10,
  mentor: 4000,
  approval: 500,
} as const;

/** The template changes only with a new release. */
export const templateQuery = queryOptions({
  queryKey: ['incubator', 'template'],
  queryFn: () => unwrap(api.GET('/incubator/template')),
  staleTime: Number.POSITIVE_INFINITY,
});

export function stepsQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'steps'],
    queryFn: () =>
      unwrap(api.GET('/projects/{projectId}/steps', { params: { path: { projectId } } })),
  });
}

/** One step with its ETag. The editor owns the values after loading. */
export function stepQuery(projectId: string, stepKey: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'step', stepKey],
    queryFn: () =>
      unwrapTagged(
        api.GET('/projects/{projectId}/steps/{stepKey}', {
          params: { path: { projectId, stepKey } },
        }),
      ),
  });
}

export function fetchStep(projectId: string, stepKey: string) {
  return unwrapTagged(
    api.GET('/projects/{projectId}/steps/{stepKey}', { params: { path: { projectId, stepKey } } }),
  );
}

export function saveStep(
  projectId: string,
  stepKey: string,
  body: components['schemas']['StepUpdate'],
  etag: string,
) {
  return unwrapTagged(
    api.PATCH('/projects/{projectId}/steps/{stepKey}', {
      params: { path: { projectId, stepKey }, header: { 'If-Match': etag } },
      body,
    }),
  );
}

type StepAction = 'check' | 'complete';

/** Runs a step action and stores the returned step in the cache. */
export function useStepAction(projectId: string, stepKey: string, action: StepAction) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      unwrapTagged(
        action === 'check'
          ? api.POST('/projects/{projectId}/steps/{stepKey}/check', {
              params: { path: { projectId, stepKey } },
            })
          : api.POST('/projects/{projectId}/steps/{stepKey}/complete', {
              params: { path: { projectId, stepKey } },
            }),
      ),
    onSuccess: (result) => {
      queryClient.setQueryData(stepQuery(projectId, stepKey).queryKey, result);
      void queryClient.invalidateQueries({ queryKey: stepsQuery(projectId).queryKey });
      void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'card'] });
      void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'activity'] });
    },
  });
}

export function useApproveStep(projectId: string, stepKey: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (comment: string) =>
      unwrapTagged(
        api.POST('/projects/{projectId}/steps/{stepKey}/approve', {
          params: { path: { projectId, stepKey } },
          body: { comment },
        }),
      ),
    onSuccess: (result) => {
      queryClient.setQueryData(stepQuery(projectId, stepKey).queryKey, result);
      void queryClient.invalidateQueries({ queryKey: stepsQuery(projectId).queryKey });
      void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'card'] });
      void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'activity'] });
    },
  });
}

export function revisionsQuery(projectId: string, stepKey: string, field: string) {
  return infiniteQueryOptions({
    queryKey: ['projects', projectId, 'revisions', stepKey, field],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/projects/{projectId}/steps/{stepKey}/revisions', {
          params: {
            path: { projectId, stepKey },
            query: { field, limit: 20, ...(pageParam ? { cursor: pageParam } : {}) },
          },
        }),
      ),
    initialPageParam: '',
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function restoreRevision(
  projectId: string,
  stepKey: string,
  revisionId: string,
  etag: string,
) {
  return unwrapTagged(
    api.POST('/projects/{projectId}/steps/{stepKey}/revisions/{revisionId}/restore', {
      params: { path: { projectId, stepKey, revisionId }, header: { 'If-Match': etag } },
    }),
  );
}

export function commentsQuery(projectId: string, stepKey: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'comments', stepKey],
    queryFn: async () => {
      const items: Comment[] = [];
      let cursor: string | undefined;
      // Comments of a step are few; read them all so threads are complete.
      for (let page = 0; page < 10; page++) {
        const result = await unwrap(
          api.GET('/projects/{projectId}/comments', {
            params: {
              path: { projectId },
              query: { step_key: stepKey, limit: 100, ...(cursor ? { cursor } : {}) },
            },
          }),
        );
        items.push(...result.items);
        if (!result.next_cursor) break;
        cursor = result.next_cursor;
      }
      return items;
    },
  });
}

function useInvalidateComments(projectId: string, stepKey: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: commentsQuery(projectId, stepKey).queryKey }),
      queryClient.invalidateQueries({ queryKey: stepsQuery(projectId).queryKey }),
      queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'activity'] }),
    ]);
}

export function useCreateComment(projectId: string, stepKey: string) {
  const invalidate = useInvalidateComments(projectId, stepKey);
  return useMutation({
    mutationFn: (body: CommentCreate) =>
      unwrap(
        api.POST('/projects/{projectId}/comments', {
          params: {
            path: { projectId },
            header: { 'Idempotency-Key': crypto.randomUUID() },
          },
          body,
        }),
      ),
    onSuccess: invalidate,
  });
}

export function useUpdateComment(projectId: string, stepKey: string) {
  const invalidate = useInvalidateComments(projectId, stepKey);
  return useMutation({
    mutationFn: ({
      commentId,
      body,
    }: {
      commentId: string;
      body: components['schemas']['CommentUpdate'];
    }) =>
      unwrap(
        api.PATCH('/projects/{projectId}/comments/{commentId}', {
          params: { path: { projectId, commentId } },
          body,
        }),
      ),
    onSuccess: invalidate,
  });
}

export function useDeleteComment(projectId: string, stepKey: string) {
  const invalidate = useInvalidateComments(projectId, stepKey);
  return useMutation({
    mutationFn: (commentId: string) =>
      unwrap(
        api.DELETE('/projects/{projectId}/comments/{commentId}', {
          params: { path: { projectId, commentId } },
        }),
      ),
    onSuccess: invalidate,
  });
}

export function mentorQuery(projectId: string, stepKey: string) {
  return infiniteQueryOptions({
    queryKey: ['projects', projectId, 'mentor', stepKey],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/projects/{projectId}/steps/{stepKey}/mentor', {
          params: {
            path: { projectId, stepKey },
            query: { limit: 30, ...(pageParam ? { cursor: pageParam } : {}) },
          },
        }),
      ),
    initialPageParam: '',
    // Pages go back in time: the next one holds earlier messages.
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function materialsQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'materials'],
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/projects/{projectId}/materials', { params: { path: { projectId } } }),
        )
      ).items,
    // Generation runs in the background: poll while anything is pending.
    refetchInterval: (query) =>
      query.state.data?.some((m) => m.status === 'pending') ? 3_000 : false,
  });
}

export function useCreateMaterial(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: components['schemas']['MaterialCreate']) =>
      unwrap(
        api.POST('/projects/{projectId}/materials', {
          params: { path: { projectId }, header: { 'Idempotency-Key': crypto.randomUUID() } },
          body,
        }),
      ),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: materialsQuery(projectId).queryKey }),
  });
}
