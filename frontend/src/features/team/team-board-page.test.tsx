import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { loadNamespaces } from '@/shared/i18n';

import { TeamBoardPage, type TeamSearch } from './team-board-page';

const server = setupServer();
beforeAll(async () => {
  server.listen({ onUnhandledFrame: 'error' });
  await loadNamespaces('team');
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  server.close();
});

function renderBoard(search: TeamSearch) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const routeTree = root.addChildren([
    createRoute({
      getParentRoute: () => root,
      path: '/',
      component: () => <TeamBoardPage search={search} onSearchChange={() => undefined} />,
    }),
  ]);
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

describe('TeamBoardPage', () => {
  it.each([{}, { tab: 'mine' as const }])(
    'puts the board in the panel of the selected tab (%o)',
    async (search) => {
      server.use(
        http.get('*/api/v1/team-requests', () =>
          HttpResponse.json({ items: [], next_cursor: null }),
        ),
      );
      renderBoard(search);
      const selected = await screen.findByRole('tab', { selected: true });
      const panel = screen.getByRole('tabpanel');
      // The selected tab names the panel that is really on the page (axe: aria-valid-attr-value).
      expect(selected.getAttribute('aria-controls')).toBe(panel.id);
      expect(panel).toHaveAttribute('aria-labelledby', selected.id);
    },
  );
});
