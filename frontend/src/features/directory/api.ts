import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';

import { tenantQuery } from '@/features/auth/api';
import { api, unwrap, unwrapTagged } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type DirectorySettings = components['schemas']['DirectorySettings'];
export type DirectorySettingsUpdate = components['schemas']['DirectorySettingsUpdate'];
export type DirectoryPreset = components['schemas']['DirectoryPreset'];
export type DirectoryPresetDefaults = components['schemas']['DirectoryPresetDefaults'];
export type DirectoryConnectionTest = components['schemas']['DirectoryConnectionTest'];
export type DirectoryTestStep = components['schemas']['DirectoryTestStep'];
export type DirectoryUserTest = components['schemas']['DirectoryUserTest'];
export type DirectoryRoleMapping = components['schemas']['DirectoryRoleMapping'];

export const DIRECTORY_PRESETS = [
  'active_directory',
  'openldap',
  'freeipa',
  'custom',
] as const satisfies readonly DirectoryPreset[];

/** Limits enforced again by the server (openapi.yaml). */
export const DIRECTORY_LIMITS = {
  url: 1000,
  dn: 1024,
  filter: 1024,
  attribute: 64,
  ca: 65536,
  password: 256,
  mappings: 50,
  label: 80,
  hint: 160,
  username: 256,
} as const;

export const directorySettingsQuery = queryOptions({
  queryKey: ['admin', 'directory', 'settings'],
  queryFn: () => unwrapTagged(api.GET('/admin/directory')),
});

export function useSaveDirectorySettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ body, etag }: { body: DirectorySettingsUpdate; etag: string | undefined }) =>
      unwrapTagged(
        api.PUT('/admin/directory', {
          body,
          params: { header: etag ? { 'If-Match': etag } : {} },
        }),
      ),
    onSuccess: (saved) => {
      queryClient.setQueryData(directorySettingsQuery.queryKey, saved);
      // The sign-in page reads whether the directory is offered.
      void queryClient.invalidateQueries({ queryKey: tenantQuery.queryKey });
    },
  });
}

export function useTestDirectoryConnection() {
  return useMutation({
    mutationFn: (body: DirectorySettingsUpdate) =>
      unwrap(api.POST('/admin/directory/test', { body })),
  });
}

export function useTestDirectoryUser() {
  return useMutation({
    mutationFn: (body: { settings: DirectorySettingsUpdate; username: string }) =>
      unwrap(api.POST('/admin/directory/test-user', { body })),
  });
}
