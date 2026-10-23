import type { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, lazyRouteComponent } from '@tanstack/react-router';

import { RootLayout } from '@/app/root-layout';

export interface RouterContext {
  queryClient: QueryClient;
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  // Loaded on demand: its drawings would otherwise sit in the entry chunk of every page.
  notFoundComponent: lazyRouteComponent(
    () => import('@/features/system/not-found-page'),
    'NotFoundPage',
  ),
});
