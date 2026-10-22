import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRouter, RouterProvider } from '@tanstack/react-router';

import { sessionQuery } from '@/features/auth/api';
import { routeTree } from '@/routeTree.gen';
import { ApiError, setUnauthenticatedHandler } from '@/shared/api/client';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      // Client errors (4xx) are final; only transient failures are retried.
      retry: (failureCount, error) =>
        !(error instanceof ApiError && error.status < 500) && failureCount < 2,
    },
  },
});

const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: 'intent',
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
  interface StaticDataRouteOption {
    /** Page title shown in the top bar of the signed-in area (`app:titles.*`). */
    titleKey?:
      | 'home'
      | 'profile'
      | 'team'
      | 'security'
      | 'members'
      | 'import'
      | 'ai'
      | 'aiUsage'
      | 'knowledge'
      | 'projects'
      | 'rubric'
      | 'showcaseReview'
      | 'contacts';
    /** The page brings its own columns: no content padding (project workspace). */
    bleed?: boolean;
  }
}

// A session lost while working (expired, revoked in another tab): forget it and
// return to sign-in. Initial visits are handled by the route guards instead.
setUnauthenticatedHandler(() => {
  const hadSession = Boolean(queryClient.getQueryState(sessionQuery.queryKey)?.data);
  queryClient.setQueryData(sessionQuery.queryKey, null);
  const { pathname, search } = window.location;
  if (hadSession && (pathname.startsWith('/app') || pathname.startsWith('/onboarding'))) {
    void router.navigate({ to: '/login', search: { redirect: pathname + search } });
  }
});

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
