import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import type { ReactNode } from 'react';
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest';

import { useMentor } from './hooks';

const server = setupServer();
beforeAll(() => {
  server.listen({ onUnhandledFrame: 'error' });
});
afterEach(() => {
  server.resetHandlers();
});
afterAll(() => {
  server.close();
});

const MENTOR = '*/api/v1/projects/p1/steps/problem/mentor';

function renderMentor() {
  const client = new QueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return renderHook(() => useMentor('p1', 'problem'), { wrapper });
}

const question = {
  id: '0190a000-0000-7000-8000-0000000000aa',
  role: 'user',
  author: null,
  content: 'Как проверить проблему?',
  status: 'ok',
  sources: [],
  created_at: '2026-10-23T10:00:00Z',
};

describe('useMentor', () => {
  it('gives the question back when the server refused it', async () => {
    server.use(
      http.post(MENTOR, () =>
        HttpResponse.json(
          { type: 'about:blank', title: 'limit', status: 429, code: 'ai.limit_reached' },
          { status: 429 },
        ),
      ),
    );
    const { result } = renderMentor();
    const onUnsent = vi.fn();
    act(() => {
      result.current.send('Как проверить проблему?', undefined, onUnsent);
    });
    await waitFor(() => {
      expect(result.current.stream.status).toBe('failed');
    });
    expect(onUnsent).toHaveBeenCalledWith('Как проверить проблему?');
  });

  it('gives the question back when the network is gone', async () => {
    server.use(http.post(MENTOR, () => HttpResponse.error()));
    const { result } = renderMentor();
    const onUnsent = vi.fn();
    act(() => {
      result.current.send('Вопрос', undefined, onUnsent);
    });
    await waitFor(() => {
      expect(onUnsent).toHaveBeenCalledWith('Вопрос');
    });
  });

  it('keeps it sent once the server has stored it, even if the answer fails', async () => {
    server.use(
      http.post(
        MENTOR,
        () =>
          new HttpResponse(
            `event: message\ndata: ${JSON.stringify(question)}\n\n` +
              'event: error\ndata: {"type":"about:blank","title":"x","status":502,"code":"ai.unavailable"}\n\n',
            { headers: { 'Content-Type': 'text/event-stream' } },
          ),
      ),
    );
    const { result } = renderMentor();
    const onUnsent = vi.fn();
    act(() => {
      result.current.send('Как проверить проблему?', undefined, onUnsent);
    });
    await waitFor(() => {
      expect(result.current.stream.status).toBe('failed');
    });
    expect(onUnsent).not.toHaveBeenCalled();
  });
});
