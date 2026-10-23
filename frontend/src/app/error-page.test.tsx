import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '@/shared/api/client';

import { RouteErrorPage } from './error-page';

function renderWithLoader(loader: () => Promise<string>) {
  const root = createRootRoute({ component: () => <Outlet /> });
  const page = createRoute({
    getParentRoute: () => root,
    path: '/',
    loader,
    component: () => <p>{page.useLoaderData()}</p>,
  });
  const router = createRouter({
    routeTree: root.addChildren([page]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
    defaultErrorComponent: RouteErrorPage,
  });
  render(<RouterProvider router={router} />);
}

describe('RouteErrorPage', () => {
  it('explains a failed load in the interface language and tries again', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const loader = vi
      .fn<() => Promise<string>>()
      .mockRejectedValueOnce(
        new ApiError({ type: 'about:blank', title: 'x', status: 503, code: 'internal' }),
      )
      .mockResolvedValue('loaded');
    renderWithLoader(loader);

    expect(
      await screen.findByRole('heading', { name: 'Не удалось открыть страницу' }),
    ).toBeVisible();
    expect(document.title).toBe('Не удалось открыть страницу — Qadam');

    await userEvent.click(screen.getByRole('button', { name: 'Повторить' }));
    expect(await screen.findByText('loaded')).toBeVisible();
    expect(loader).toHaveBeenCalledTimes(2);
  });

  it('offers a fresh page when the code of the screen is gone', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    renderWithLoader(() =>
      Promise.reject(new TypeError('Failed to fetch dynamically imported module: /x.js')),
    );
    expect(await screen.findByRole('heading', { name: 'Вышла новая версия Qadam' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/');
  });
});
