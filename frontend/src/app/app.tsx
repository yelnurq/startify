import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createRouter, RouterProvider } from '@tanstack/react-router';

import { sessionQuery } from '@/features/auth/api';
import { abandonEditors, discardDraftsExcept } from '@/features/incubator/drafts';
import { routeTree } from '@/routeTree.gen';
import { ApiError, setUnauthenticatedHandler } from '@/shared/api/client';

import { releaseBoot } from './boot';
import { installChunkReload } from './chunk-reload';
import { RouteErrorPage } from './error-page';
import { forgetUserData, watchSessionUser } from './session-watch';

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
  defaultErrorComponent: RouteErrorPage,
  scrollRestoration: true,
});

// A page left open across a deployment reloads once to get the new code.
installChunkReload();

// The server-drawn first screen of public pages gives way once the first page
// is drawn with its code and data.
releaseBoot(
  () =>
    router.state.status === 'idle' &&
    !router.state.isLoading &&
    queryClient.isFetching() === 0 &&
    (document.querySelector('#root main')?.childElementCount ?? 0) > 0,
);

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

// Whoever signs in next at this computer sees nothing cached for the previous
// user. Step editors still saving stop with the session; unsaved drafts stay
// only for the person who wrote them.
watchSessionUser(queryClient, (user) => {
  abandonEditors();
  if (user !== null) discardDraftsExcept(user);
});

// A session lost while working (expired, revoked in another tab): forget it and
// everything fetched with it, and return to sign-in. Initial visits are handled
// by the route guards instead.
setUnauthenticatedHandler(() => {
  const hadSession = Boolean(queryClient.getQueryState(sessionQuery.queryKey)?.data);
  forgetUserData(queryClient);
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
