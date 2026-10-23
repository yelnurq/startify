import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { usePresence } from './hooks';

const server = setupServer();
beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
  // jsdom does not draw, so its page is never "visible" by default.
  Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true });
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  server.close();
});

/** Waits with plain timeouts (the intervals are mocked in the test). */
async function until(condition: () => boolean) {
  for (let i = 0; i < 100 && !condition(); i++) {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

describe('usePresence', () => {
  it('stops reporting once the project turns out to be missing', async () => {
    const ticks: (() => void)[] = [];
    const cleared: unknown[] = [];
    vi.spyOn(window, 'setInterval').mockImplementation(((callback: () => void) => {
      ticks.push(callback);
      return ticks.length;
    }) as typeof window.setInterval);
    vi.spyOn(window, 'clearInterval').mockImplementation((id) => {
      cleared.push(id);
    });
    let calls = 0;
    server.use(
      http.post('*/api/v1/projects/gone/presence', () => {
        calls++;
        return HttpResponse.json(
          { type: 'about:blank', title: 'x', status: 404, code: 'project.not_found' },
          { status: 404 },
        );
      }),
    );
    renderHook(() => usePresence('gone', undefined));
    await until(() => cleared.length > 0);
    expect(calls).toBe(1);
    expect(cleared).toEqual([1]);
    // A late tick of the stopped timer reports nothing.
    ticks[0]?.();
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(calls).toBe(1);
  });

  it('does not report for a project that has not opened', async () => {
    let calls = 0;
    server.use(
      http.post('*/api/v1/projects/p1/presence', () => {
        calls++;
        return HttpResponse.json({ viewers: [], versions: {} });
      }),
    );
    const { rerender } = renderHook(({ enabled }) => usePresence('p1', 'problem', enabled), {
      initialProps: { enabled: false },
    });
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(calls).toBe(0);
    rerender({ enabled: true });
    await waitFor(() => {
      expect(calls).toBeGreaterThan(0);
    });
  });
});
