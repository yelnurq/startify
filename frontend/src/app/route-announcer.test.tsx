import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { render, renderHook, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { setRouteTitle, useDocumentMeta } from '@/shared/hooks/use-document-meta';

import { RouteAnnouncer } from './route-announcer';
import { focusMainHeading } from './route-focus';

afterEach(() => {
  document.body.innerHTML = '';
  setRouteTitle(undefined);
});

describe('focusMainHeading', () => {
  it('focuses the heading of the page once it is drawn', async () => {
    document.body.innerHTML = '<main id="main"></main>';
    const missing = vi.fn();
    focusMainHeading(missing);
    const heading = document.createElement('h1');
    heading.textContent = 'Проекты';
    document.querySelector('main')?.append(heading);
    await waitFor(() => {
      expect(document.activeElement).toBe(heading);
    });
    expect(heading.tabIndex).toBe(-1);
    expect(missing).not.toHaveBeenCalled();
  });

  it('leaves the focus the person moved into a field', () => {
    document.body.innerHTML = '<main><input aria-label="q" /><h1>Витрина</h1></main>';
    const input = document.querySelector('input');
    input?.focus();
    focusMainHeading(() => undefined);
    // The field was focused before the navigation started: the heading takes over.
    expect(document.activeElement?.tagName).toBe('H1');

    document.body.innerHTML = '<main id="main"></main>';
    focusMainHeading(() => undefined);
    const field = document.createElement('input');
    document.querySelector('main')?.append(field);
    field.focus();
    const heading = document.createElement('h1');
    document.querySelector('main')?.append(heading);
    expect(document.activeElement).toBe(field);
  });

  it('reports a page without a heading', () => {
    vi.useFakeTimers();
    document.body.innerHTML = '<main id="main"><p>…</p></main>';
    const missing = vi.fn();
    focusMainHeading(missing);
    vi.advanceTimersByTime(3_000);
    expect(missing).toHaveBeenCalledOnce();
    vi.useRealTimers();
  });
});

describe('document title', () => {
  it('shows the page’s own title over the route’s while the page is open', () => {
    setRouteTitle('Проекты — Qadam');
    expect(document.title).toBe('Проекты — Qadam');
    const page = renderHook(
      ({ title }) => {
        useDocumentMeta(title);
      },
      { initialProps: { title: 'AgroScan — Qadam' } },
    );
    expect(document.title).toBe('AgroScan — Qadam');
    setRouteTitle('Проекты — Qadam');
    expect(document.title).toBe('AgroScan — Qadam');
    page.unmount();
    expect(document.title).toBe('Проекты — Qadam');
    setRouteTitle(undefined);
    expect(document.title).toBe('Qadam');
  });
});

describe('RouteAnnouncer', () => {
  it('titles the routes and moves the focus to the new page’s heading', async () => {
    const root = createRootRoute({
      component: () => (
        <>
          <Link to="/login">{'go'}</Link>
          <main id="main">
            <Outlet />
          </main>
          <RouteAnnouncer />
        </>
      ),
    });
    const routeTree = root.addChildren([
      createRoute({ getParentRoute: () => root, path: '/', component: () => <p>{'home'}</p> }),
      createRoute({
        getParentRoute: () => root,
        path: '/login',
        staticData: { pageTitle: 'login' },
        component: () => <h1>{'Вход в Qadam'}</h1>,
      }),
    ]);
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: ['/'] }),
    });
    render(<RouterProvider router={router} />);
    await screen.findByText('home');
    expect(document.title).toBe('Qadam');

    await userEvent.click(screen.getByRole('link', { name: 'go' }));
    const heading = await screen.findByRole('heading', { name: 'Вход в Qadam' });
    await waitFor(() => {
      expect(document.activeElement).toBe(heading);
    });
    expect(document.title).toBe('Вход — Qadam');
  });
});
