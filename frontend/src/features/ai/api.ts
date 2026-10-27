import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';

import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type AiFeature = components['schemas']['AiFeature'];
export type AiRoute = components['schemas']['AiRoute'];
export type AiProvider = components['schemas']['AiProvider'];
export type AiSettings = components['schemas']['AiSettings'];
export type AiSettingsUpdate = components['schemas']['AiSettingsUpdate'];
export type AiStatus = components['schemas']['AiStatus'];
export type AiCheckResult = components['schemas']['AiCheckResult'];
export type AiUsageReport = components['schemas']['AiUsageReport'];
export type MentorMode = components['schemas']['MentorMode'];
export type KnowledgeDocument = components['schemas']['KnowledgeDocument'];
export type KnowledgeHit = components['schemas']['KnowledgeHit'];

export const AI_FEATURES = [
  'mentor',
  'evaluation',
  'generation',
  'summary',
  'embedding',
] as const satisfies readonly AiFeature[];

export const MENTOR_MODES = ['coach', 'coauthor'] as const satisfies readonly MentorMode[];

/** Limits enforced again by the server. */
export const AI_LIMITS = {
  routes: 5,
  model: 120,
  dailyUserTokens: 100_000_000,
  dailyTenantTokens: 100_000_000_000,
} as const;

/** What a member needs to know before using AI (external provider, daily limit). */
export const aiStatusQuery = queryOptions({
  queryKey: ['ai', 'status'],
  queryFn: () => unwrap(api.GET('/ai/status')),
  staleTime: 60_000,
});

export const aiSettingsQuery = queryOptions({
  queryKey: ['admin', 'ai', 'settings'],
  queryFn: () => unwrapTagged(api.GET('/admin/ai/settings')),
});

export function useSaveAiSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ body, etag }: { body: AiSettingsUpdate; etag: string | undefined }) =>
      unwrapTagged(
        api.PUT('/admin/ai/settings', {
          body,
          params: { header: etag ? { 'If-Match': etag } : {} },
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(aiSettingsQuery.queryKey, saved);
      void queryClient.invalidateQueries({ queryKey: aiStatusQuery.queryKey });
    },
  });
}

export const USAGE_PERIODS = [7, 30, 90] as const;
export type UsagePeriod = (typeof USAGE_PERIODS)[number];

export function isUsagePeriod(value: unknown): value is UsagePeriod {
  return (USAGE_PERIODS as readonly unknown[]).includes(value);
}

export function aiUsageQuery(days: UsagePeriod) {
  return queryOptions({
    queryKey: ['admin', 'ai', 'usage', days],
    queryFn: () => unwrap(api.GET('/admin/ai/usage', { params: { query: { days } } })),
  });
}

/** PDF, DOCX, TXT or Markdown up to 20 MiB (checked again by the server). */
export const KNOWLEDGE_MAX_BYTES = 20 * 1024 * 1024;
export const KNOWLEDGE_EXTENSIONS = ['.pdf', '.docx', '.txt', '.md'];
export const KNOWLEDGE_LIMITS = {
  title: 200,
  textBytes: 4 * 1024 * 1024,
  query: [2, 500],
} as const;

const KNOWLEDGE_PAGE = 20;

export const knowledgeDocumentsQuery = infiniteQueryOptions({
  queryKey: ['admin', 'knowledge', 'list'],
  queryFn: ({ pageParam }) =>
    unwrap(
      api.GET('/admin/knowledge', {
        params: { query: { limit: KNOWLEDGE_PAGE, ...(pageParam ? { cursor: pageParam } : {}) } },
      }),
    ),
  initialPageParam: '',
  getNextPageParam: (page) => page.next_cursor ?? undefined,
  // Documents are indexed in the background: poll until none is pending.
  refetchInterval: (query) =>
    query.state.data?.pages.some((page) => page.items.some((doc) => doc.status === 'pending'))
      ? 3_000
      : false,
});

export type KnowledgeSource = { kind: 'text'; text: string } | { kind: 'file'; file: File };

export function useCreateKnowledgeDocument() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ title, source }: { title: string; source: KnowledgeSource }) => {
      const form = new FormData();
      form.append('title', title);
      if (source.kind === 'text') form.append('text', source.text);
      else form.append('file', source.file);
      return unwrap(
        api.POST('/admin/knowledge', {
          // The typed body only names the fields; the request carries the form itself.
          body: { title },
          bodySerializer: () => form,
        }),
      );
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'knowledge'] }),
  });
}

export function useDeleteKnowledgeDocument() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (documentId: string) =>
      unwrap(api.DELETE('/admin/knowledge/{documentId}', { params: { path: { documentId } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'knowledge'] }),
  });
}

export function knowledgeSearchQuery(q: string) {
  return queryOptions({
    queryKey: ['admin', 'knowledge', 'search', q],
    queryFn: async () =>
      (await unwrap(api.GET('/admin/knowledge/search', { params: { query: { q, limit: 5 } } })))
        .items,
    enabled: q !== '',
  });
}
