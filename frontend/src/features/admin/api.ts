import { infiniteQueryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import type { Role } from '@/features/auth/api';
import { api, unwrap } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type Member = components['schemas']['Member'];
export type MemberUpdate = components['schemas']['MemberUpdate'];
export type ImportKind = components['schemas']['ImportKind'];
export type ImportJob = components['schemas']['ImportJob'];
export type ImportRow = components['schemas']['ImportRow'];
export type ImportResult = components['schemas']['ImportResult'];

export const ROLES = [
  'student',
  'supervisor',
  'mentor',
  'jury',
  'innovation_manager',
  'tenant_admin',
  'leadership',
  'partner',
] as const satisfies readonly Role[];

export function isRole(value: unknown): value is Role {
  return typeof value === 'string' && (ROLES as readonly string[]).includes(value);
}

export interface MemberFilters {
  q?: string;
  role?: Role;
  faculty_id?: string;
}

export function membersQuery(filters: MemberFilters) {
  return infiniteQueryOptions({
    queryKey: ['admin', 'members', filters],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/admin/members', {
          params: { query: { ...filters, limit: 25, ...(pageParam ? { cursor: pageParam } : {}) } },
        }),
      ),
    initialPageParam: '',
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });
}

export function useUpdateMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, body }: { userId: string; body: MemberUpdate }) =>
      unwrap(api.PATCH('/admin/members/{userId}', { params: { path: { userId } }, body })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'members'] }),
  });
}

/** CSV or XLSX up to 10 MiB and 3000 rows (checked again by the server). */
export const IMPORT_MAX_BYTES = 10 * 1024 * 1024;
export const IMPORT_EXTENSIONS = ['.csv', '.xlsx'];

export function useCreateImport() {
  return useMutation({
    mutationFn: ({ kind, file }: { kind: ImportKind; file: File }) => {
      const form = new FormData();
      form.append('kind', kind);
      form.append('file', file);
      return unwrap(
        api.POST('/admin/imports', {
          // The typed body only names the fields; the request carries the file itself.
          body: { kind, file: file.name },
          bodySerializer: () => form,
        }),
      );
    },
  });
}

export function useCommitImport() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (importId: string) =>
      unwrap(api.POST('/admin/imports/{importId}/commit', { params: { path: { importId } } })),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'members'] });
      void queryClient.invalidateQueries({ queryKey: ['faculties'] });
      void queryClient.invalidateQueries({ queryKey: ['study-groups'] });
    },
  });
}
