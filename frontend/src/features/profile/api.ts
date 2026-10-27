import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { sessionQuery, type Session } from '@/features/auth/api';
import { api, unwrap } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type Me = components['schemas']['Me'];
export type Profile = components['schemas']['Profile'];
export type ProfileUpdate = components['schemas']['ProfileUpdate'];
export type PublicProfile = components['schemas']['PublicProfile'];

export const meQuery = queryOptions({
  queryKey: ['me'],
  queryFn: () => unwrap(api.GET('/me')),
});

export function publicProfileQuery(slug: string) {
  return queryOptions({
    queryKey: ['profiles', slug],
    queryFn: () => unwrap(api.GET('/profiles/{slug}', { params: { path: { slug } } })),
  });
}

/** Stores a fresh `Me` and keeps the session (shell, guards) in step with it. */
export function useApplyMe() {
  const queryClient = useQueryClient();
  return (me: Me) => {
    queryClient.setQueryData(meQuery.queryKey, me);
    queryClient.setQueryData<Session | null>(sessionQuery.queryKey, (session) =>
      session ? { ...session, user: me.user, membership: me.membership } : session,
    );
  };
}

export function useUpdateMe() {
  const apply = useApplyMe();
  return useMutation({
    mutationFn: (body: components['schemas']['MeUpdate']) => unwrap(api.PATCH('/me', { body })),
    onSuccess: apply,
  });
}

export function useUpdateProfile() {
  const apply = useApplyMe();
  return useMutation({
    mutationFn: (body: ProfileUpdate) => unwrap(api.PUT('/me/profile', { body })),
    onSuccess: apply,
  });
}

export const AVATAR_TYPES = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
export const AVATAR_MAX_BYTES = 10 * 1024 * 1024;

export function useUploadAvatar() {
  const apply = useApplyMe();
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append('file', file);
      return unwrap(
        api.PUT('/me/avatar', {
          // The typed body only names the field; the request carries the file itself.
          body: { file: file.name },
          bodySerializer: () => form,
        }),
      );
    },
    onSuccess: apply,
  });
}

export function useDeleteAvatar() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.DELETE('/me/avatar')),
    onSuccess: () => {
      queryClient.setQueryData<Me>(meQuery.queryKey, (me) =>
        me ? { ...me, user: { ...me.user, avatar_url: null } } : me,
      );
      queryClient.setQueryData<Session | null>(sessionQuery.queryKey, (session) =>
        session ? { ...session, user: { ...session.user, avatar_url: null } } : session,
      );
    },
  });
}
