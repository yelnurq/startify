import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';

import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

import { listQuery, type ShowcaseSearch } from './logic';

export type ShowcaseEntry = components['schemas']['ShowcaseEntry'];
export type ShowcaseEntryUpdate = components['schemas']['ShowcaseEntryUpdate'];
export type ShowcaseStatus = components['schemas']['ShowcaseStatus'];
export type ShowcaseVisibility = components['schemas']['ShowcaseVisibility'];
export type ShowcaseSections = components['schemas']['ShowcaseSections'];
export type ShowcaseTeamMember = components['schemas']['ShowcaseTeamMember'];
export type ShowcaseCard = components['schemas']['ShowcaseCard'];
export type ShowcaseFacets = components['schemas']['ShowcaseFacets'];
export type ShowcaseHighlights = components['schemas']['ShowcaseHighlights'];
export type ShowcaseProject = components['schemas']['ShowcaseProject'];
export type UniversityBrief = components['schemas']['UniversityBrief'];
export type ContactCreate = components['schemas']['ContactCreate'];
export type ContactRequest = components['schemas']['ContactRequest'];
export type ContactStatus = components['schemas']['ContactStatus'];

/** Cards per page of the catalogue (a multiple of the 1, 2 and 3 columns). */
export const PAGE_SIZE = 12;

export function showcaseListQuery(search: ShowcaseSearch) {
  const query = listQuery(search);
  return infiniteQueryOptions({
    queryKey: ['showcase', 'list', query],
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/showcase', {
          params: {
            query: { ...query, limit: PAGE_SIZE, ...(pageParam ? { cursor: pageParam } : {}) },
          },
        }),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
}

export function showcaseFacetsQuery(national: boolean) {
  return queryOptions({
    queryKey: ['showcase', 'facets', national ? 'national' : 'university'],
    queryFn: () =>
      unwrap(
        api.GET('/showcase/facets', {
          params: { query: { scope: national ? 'national' : 'university' } },
        }),
      ),
    staleTime: 60_000,
  });
}

export const showcaseHighlightsQuery = queryOptions({
  queryKey: ['showcase', 'highlights'],
  queryFn: () => unwrap(api.GET('/showcase/highlights')),
  staleTime: 60_000,
});

export function showcaseProjectQuery(slug: string) {
  return queryOptions({
    queryKey: ['showcase', 'entry', slug],
    queryFn: () => unwrap(api.GET('/showcase/entries/{slug}', { params: { path: { slug } } })),
  });
}

export function useContactTeam(slug: string) {
  return useMutation({
    mutationFn: (body: ContactCreate) =>
      unwrap(api.POST('/showcase/entries/{slug}/contact', { params: { path: { slug } }, body })),
  });
}

// The team's page.

export function showcaseEntryQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'showcase'],
    queryFn: () =>
      unwrapTagged(api.GET('/projects/{projectId}/showcase', { params: { path: { projectId } } })),
  });
}

export function projectContactsQuery(projectId: string) {
  return queryOptions({
    queryKey: ['projects', projectId, 'contacts'],
    queryFn: async () =>
      (await unwrap(api.GET('/projects/{projectId}/contacts', { params: { path: { projectId } } })))
        .items,
  });
}

/**
 * Keeps the cached page and the catalogue in step with a change. Covers and
 * status changes return no ETag, so the page is re-read for the next If-Match.
 */
function useEntryChanged(projectId: string) {
  const queryClient = useQueryClient();
  return (entry: ShowcaseEntry, etag?: string) => {
    const key = showcaseEntryQuery(projectId).queryKey;
    if (etag !== undefined) {
      queryClient.setQueryData(key, { data: entry, etag });
    } else {
      queryClient.setQueryData(key, (old) => (old ? { ...old, data: entry } : old));
      void queryClient.invalidateQueries({ queryKey: key });
    }
    void queryClient.invalidateQueries({ queryKey: ['showcase'] });
    void queryClient.invalidateQueries({ queryKey: ['admin', 'showcase'] });
  };
}

export function useSaveShowcaseEntry(projectId: string) {
  const changed = useEntryChanged(projectId);
  return useMutation({
    mutationFn: ({ body, etag }: { body: ShowcaseEntryUpdate; etag: string | undefined }) =>
      unwrapTagged(
        api.PUT('/projects/{projectId}/showcase', {
          params: { path: { projectId }, header: etag ? { 'If-Match': etag } : {} },
          body,
        }),
      ),
    onSuccess: ({ data, etag }) => {
      changed(data, etag);
    },
  });
}

export function useSetShowcaseCover(projectId: string) {
  const changed = useEntryChanged(projectId);
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append('file', file);
      return unwrap(
        api.PUT('/projects/{projectId}/showcase/cover', {
          params: { path: { projectId } },
          // The typed body only names the field; the request carries the file itself.
          body: { file: file.name },
          bodySerializer: () => form,
        }),
      );
    },
    onSuccess: (entry) => {
      changed(entry);
    },
  });
}

export function useDeleteShowcaseCover(projectId: string) {
  const changed = useEntryChanged(projectId);
  return useMutation({
    mutationFn: () =>
      unwrap(
        api.DELETE('/projects/{projectId}/showcase/cover', { params: { path: { projectId } } }),
      ),
    onSuccess: (entry) => {
      changed(entry);
    },
  });
}

export function usePublishShowcaseEntry(projectId: string) {
  const changed = useEntryChanged(projectId);
  return useMutation({
    mutationFn: (publish: boolean) =>
      unwrap(
        publish
          ? api.POST('/projects/{projectId}/showcase/publish', { params: { path: { projectId } } })
          : api.POST('/projects/{projectId}/showcase/unpublish', {
              params: { path: { projectId } },
            }),
      ),
    onSuccess: (entry) => {
      changed(entry);
    },
  });
}

// Moderation by the innovation office.

export function showcaseReviewsQuery(status: ShowcaseStatus) {
  return queryOptions({
    queryKey: ['admin', 'showcase', status],
    queryFn: async () =>
      (await unwrap(api.GET('/admin/showcase', { params: { query: { status } } }))).items,
  });
}

export function useReviewShowcaseEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      projectId,
      approve,
      comment,
    }: {
      projectId: string;
      approve: boolean;
      comment?: string;
    }) =>
      unwrap(
        api.POST('/admin/showcase/{projectId}/review', {
          params: { path: { projectId } },
          body: comment ? { approve, comment } : { approve },
        }),
      ),
    onSettled: (_entry, _error, { projectId }) => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'showcase'] });
      void queryClient.invalidateQueries({ queryKey: ['showcase'] });
      void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'showcase'] });
    },
  });
}

export function contactRequestsQuery(status: ContactStatus) {
  return queryOptions({
    queryKey: ['admin', 'contacts', status],
    queryFn: async () =>
      (await unwrap(api.GET('/admin/contacts', { params: { query: { status } } }))).items,
  });
}

export function useModerateContact() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ contactId, forward }: { contactId: string; forward: boolean }) =>
      unwrap(
        api.POST('/admin/contacts/{contactId}/moderate', {
          params: { path: { contactId } },
          body: { forward },
        }),
      ),
    onSettled: (contact) => {
      void queryClient.invalidateQueries({ queryKey: ['admin', 'contacts'] });
      if (contact) {
        void queryClient.invalidateQueries({
          queryKey: ['projects', contact.project_id, 'contacts'],
        });
      }
    },
  });
}
