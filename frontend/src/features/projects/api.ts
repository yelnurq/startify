import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';

import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type Project = components['schemas']['Project'];
export type ProjectSummary = components['schemas']['ProjectSummary'];
export type ProjectCreate = components['schemas']['ProjectCreate'];
export type ProjectUpdate = components['schemas']['ProjectUpdate'];
export type ProjectMember = components['schemas']['ProjectMember'];
export type ProjectRole = components['schemas']['ProjectRole'];
export type ProjectAccess = components['schemas']['ProjectAccess'];
export type ProjectProgress = components['schemas']['ProjectProgress'];
export type Industry = components['schemas']['Industry'];
export type Badge = components['schemas']['Badge'];
export type Activity = components['schemas']['Activity'];
export type Presence = components['schemas']['Presence'];
export type Person = components['schemas']['Person'];
export type MemberBrief = components['schemas']['MemberBrief'];

export const INDUSTRIES = [
  'agritech',
  'edtech',
  'fintech',
  'healthtech',
  'logistics',
  'cleantech',
  'govtech',
  'foodtech',
  'fashion',
  'ecommerce',
  'manufacturing',
  'energy',
  'tourism',
  'social',
  'other',
] as const satisfies readonly Industry[];

export const BADGES = [
  'idea_stage',
  'market_stage',
  'product_stage',
  'launch_stage',
  'first_review',
  'materials_ready',
  'team_of_three',
] as const satisfies readonly Badge[];

export function isIndustry(value: unknown): value is Industry {
  return typeof value === 'string' && (INDUSTRIES as readonly string[]).includes(value);
}

/** Limits checked again by the server. */
export const PROJECT_LIMITS = { title: [2, 120], tagline: 160, memberTitle: 60 } as const;

export type ProjectScope = 'mine' | 'all';

const PAGE_SIZE = 12;

export function projectsQuery(scope: ProjectScope, q?: string) {
  return infiniteQueryOptions({
    queryKey: ['projects', 'list', scope, q ?? ''],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/projects', {
          params: {
            query: {
              scope,
              limit: PAGE_SIZE,
              ...(q ? { q } : {}),
              ...(pageParam ? { cursor: pageParam } : {}),
            },
          },
        }),
      ),
    initialPageParam: '',
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

/** The member's most recently changed project, for the dashboard. */
export const latestProjectQuery = queryOptions({
  queryKey: ['projects', 'list', 'latest'],
  queryFn: async () =>
    (await unwrap(api.GET('/projects', { params: { query: { scope: 'mine', limit: 1 } } })))
      .items[0] ?? null,
});

export function projectQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'card'],
    queryFn: () =>
      unwrapTagged(api.GET('/projects/{projectId}', { params: { path: { projectId } } })),
  });
}

export function activityQuery(projectId: string, limit = 30) {
  return queryOptions({
    queryKey: ['projects', projectId, 'activity', limit],
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/projects/{projectId}/activity', {
            params: { path: { projectId }, query: { limit } },
          }),
        )
      ).items,
  });
}

export function peopleQuery(q: string, role?: components['schemas']['Role']) {
  return queryOptions({
    queryKey: ['people', q, role ?? ''],
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/people', {
            params: { query: { limit: 10, ...(q ? { q } : {}), ...(role ? { role } : {}) } },
          }),
        )
      ).items,
    staleTime: 60_000,
  });
}

export function useCreateProject() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ProjectCreate) =>
      unwrap(
        api.POST('/projects', {
          body,
          params: { header: { 'Idempotency-Key': crypto.randomUUID() } },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', 'list'] }),
  });
}

function useInvalidateProject(projectId: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['projects', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['projects', 'list'] }),
    ]);
}

export function useUpdateProject(projectId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ body, etag }: { body: ProjectUpdate; etag: string | undefined }) =>
      unwrapTagged(
        api.PATCH('/projects/{projectId}', {
          params: { path: { projectId }, header: etag ? { 'If-Match': etag } : {} },
          body,
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(projectQuery(projectId).queryKey, saved);
      void queryClient.invalidateQueries({ queryKey: ['projects', 'list'] });
    },
  });
}

export function useArchiveProject(projectId: string) {
  const invalidate = useInvalidateProject(projectId);
  return useMutation({
    mutationFn: () =>
      unwrap(api.DELETE('/projects/{projectId}', { params: { path: { projectId } } })),
    onSuccess: invalidate,
  });
}

export function useAddMember(projectId: string) {
  const invalidate = useInvalidateProject(projectId);
  return useMutation({
    mutationFn: (body: components['schemas']['ProjectMemberAdd']) =>
      unwrap(api.POST('/projects/{projectId}/members', { params: { path: { projectId } }, body })),
    onSuccess: invalidate,
  });
}

export function useUpdateMember(projectId: string) {
  const invalidate = useInvalidateProject(projectId);
  return useMutation({
    mutationFn: ({
      userId,
      body,
    }: {
      userId: string;
      body: components['schemas']['ProjectMemberUpdate'];
    }) =>
      unwrap(
        api.PATCH('/projects/{projectId}/members/{userId}', {
          params: { path: { projectId, userId } },
          body,
        }),
      ),
    onSuccess: invalidate,
  });
}

export function useRemoveMember(projectId: string) {
  const invalidate = useInvalidateProject(projectId);
  return useMutation({
    mutationFn: (userId: string) =>
      unwrap(
        api.DELETE('/projects/{projectId}/members/{userId}', {
          params: { path: { projectId, userId } },
        }),
      ),
    onSuccess: invalidate,
  });
}

/** Reports being in the project; the answer lists who else is here. */
export function reportPresence(projectId: string, stepKey: string | undefined) {
  return unwrap(
    api.POST('/projects/{projectId}/presence', {
      params: { path: { projectId } },
      body: stepKey ? { step_key: stepKey } : {},
    }),
  );
}

/** Team roles that may edit the project's answers. */
export function canEdit(project: Pick<Project, 'access' | 'archived'>): boolean {
  return !project.archived && (project.access === 'owner' || project.access === 'member');
}

/** The project's supervisor or an innovation manager may approve steps. */
export function canApprove(
  project: Pick<Project, 'access' | 'archived'>,
  innovationManager: boolean,
): boolean {
  if (project.archived) return false;
  return project.access === 'supervisor' || (project.access === 'manager' && innovationManager);
}
