import { queryOptions } from '@tanstack/react-query';

import { api, unwrap } from '@/shared/api/client';

export const metaQuery = queryOptions({
  queryKey: ['meta'],
  queryFn: () => unwrap(api.GET('/meta')),
});
