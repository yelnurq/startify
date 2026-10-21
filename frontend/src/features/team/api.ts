import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';

import { api, unwrap } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type TeamRequest = components['schemas']['TeamRequest'];
export type TeamRole = components['schemas']['TeamRole'];
export type TeamResponse = components['schemas']['TeamResponse'];
export type TeamRequestCreate = components['schemas']['TeamRequestCreate'];
export type TeamRequestUpdate = components['schemas']['TeamRequestUpdate'];

export const TEAM_ROLES = [
  'developer',
  'designer',
  'data_scientist',
  'marketer',
  'business',
  'hardware',
  'domain_expert',
  'other',
] as const satisfies readonly TeamRole[];

export function isTeamRole(value: unknown): value is TeamRole {
  return typeof value === 'string' && (TEAM_ROLES as readonly string[]).includes(value);
}

export const TEAM_LIMITS = {
  title: [5, 120],
  description: [20, 2000],
  roles: 5,
  skills: 15,
  message: [10, 1000],
} as const;

export interface TeamFilters {
  q?: string;
  role?: TeamRole;
  skill?: string;
  mine?: boolean;
}

const PAGE_SIZE = 12;

export function teamRequestsQuery(filters: TeamFilters) {
  return infiniteQueryOptions({
    queryKey: ['team-requests', 'list', filters],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/team-requests', {
          params: {
            query: { ...filters, limit: PAGE_SIZE, ...(pageParam ? { cursor: pageParam } : {}) },
          },
        }),
      ),
    initialPageParam: '',
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

/** The newest open requests for the dashboard. */
export const latestTeamRequestsQuery = queryOptions({
  queryKey: ['team-requests', 'latest'],
  queryFn: async () =>
    (await unwrap(api.GET('/team-requests', { params: { query: { limit: 3 } } }))).items,
});

export function teamRequestQuery(requestId: string) {
  return queryOptions({
    queryKey: ['team-requests', 'one', requestId],
    queryFn: () =>
      unwrap(api.GET('/team-requests/{requestId}', { params: { path: { requestId } } })),
  });
}

export function teamResponsesQuery(requestId: string) {
  return queryOptions({
    queryKey: ['team-requests', 'responses', requestId],
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/team-requests/{requestId}/responses', {
            params: { path: { requestId } },
          }),
        )
      ).items,
  });
}

function useInvalidateRequests() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: ['team-requests'] });
}

export function useCreateTeamRequest() {
  const invalidate = useInvalidateRequests();
  return useMutation({
    mutationFn: (body: TeamRequestCreate) =>
      unwrap(
        api.POST('/team-requests', {
          body,
          params: { header: { 'Idempotency-Key': crypto.randomUUID() } },
        }),
      ),
    onSuccess: invalidate,
  });
}

export function useUpdateTeamRequest() {
  const invalidate = useInvalidateRequests();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: TeamRequestUpdate }) =>
      unwrap(
        api.PATCH('/team-requests/{requestId}', { params: { path: { requestId: id } }, body }),
      ),
    onSuccess: invalidate,
  });
}

export function useRespondToTeamRequest() {
  const invalidate = useInvalidateRequests();
  return useMutation({
    mutationFn: ({ id, message }: { id: string; message: string }) =>
      unwrap(
        api.POST('/team-requests/{requestId}/responses', {
          params: {
            path: { requestId: id },
            header: { 'Idempotency-Key': crypto.randomUUID() },
          },
          body: { message },
        }),
      ),
    onSuccess: invalidate,
  });
}
