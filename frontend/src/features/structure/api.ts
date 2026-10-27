import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { api, unwrap } from '@/shared/api/client';
import type { components } from '@/shared/api/schema.gen';

export type Faculty = components['schemas']['Faculty'];
export type StudyGroup = components['schemas']['StudyGroup'];

export const facultiesQuery = queryOptions({
  queryKey: ['faculties'],
  queryFn: async () => (await unwrap(api.GET('/faculties'))).items,
  staleTime: 10 * 60_000,
});

export interface GroupSearch {
  faculty_id?: string;
  department_id?: string;
  q?: string;
}

export function studyGroupsQuery(search: GroupSearch) {
  return queryOptions({
    queryKey: ['study-groups', search],
    queryFn: async () =>
      (await unwrap(api.GET('/study-groups', { params: { query: { ...search, limit: 12 } } })))
        .items,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  });
}
