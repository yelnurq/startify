import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

import { ApiError } from './client';
import { EventStreamParser, postEventStream, readEventStream, type ServerEvent } from './sse';

describe('EventStreamParser', () => {
  it('parses named events with JSON data', () => {
    const parser = new EventStreamParser();
    expect(
      parser.push('event: delta\ndata: {"text":"Сәлем"}\n\nevent: done\ndata: {}\n\n'),
    ).toEqual([
      { event: 'delta', data: '{"text":"Сәлем"}' },
      { event: 'done', data: '{}' },
    ]);
  });

  it('joins events split across chunks, including a CRLF split in two', () => {
    const parser = new EventStreamParser();
    const events: ServerEvent[] = [];
    for (const chunk of ['eve', 'nt: delta\r', '\ndata: a', 'b\r\n', '\r\n', 'data: c\n', '\n']) {
      events.push(...parser.push(chunk));
    }
    expect(events).toEqual([
      { event: 'delta', data: 'ab' },
      { event: 'message', data: 'c' },
    ]);
  });

  it('ignores comments, unknown fields and events without data', () => {
    const parser = new EventStreamParser();
    expect(
      parser.push(': keep-alive\n\nid: 7\nretry: 100\n\nevent: x\n\ndata\ndata:  two\n\n'),
    ).toEqual([{ event: 'message', data: '\n two' }]);
  });

  it('drops an unterminated last event', () => {
    const parser = new EventStreamParser();
    expect(parser.push('event: delta\ndata: cut')).toEqual([]);
    expect(parser.end()).toEqual([]);
  });
});

function streamOf(...chunks: string[]): Response {
  const encoder = new TextEncoder();
  return new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
        controller.close();
      },
    }),
    { headers: { 'Content-Type': 'text/event-stream' } },
  );
}

describe('readEventStream', () => {
  it('delivers events as the body arrives, decoding UTF-8 split across chunks', async () => {
    const bytes = new TextEncoder().encode('data: {"text":"Қадам"}\n\n');
    const response = new Response(
      new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(bytes.slice(0, 16));
          controller.enqueue(bytes.slice(16));
          controller.close();
        },
      }),
    );
    const events: ServerEvent[] = [];
    await readEventStream(response, (event) => events.push(event));
    expect(events).toEqual([{ event: 'message', data: '{"text":"Қадам"}' }]);
  });

  it('reads a multi-event body', async () => {
    const events: ServerEvent[] = [];
    await readEventStream(streamOf('event: a\ndata: 1\n\n', 'event: b\ndata: 2\n\n'), (event) =>
      events.push(event),
    );
    expect(events.map((event) => event.event)).toEqual(['a', 'b']);
  });
});

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

describe('postEventStream', () => {
  it('posts JSON and relays the stream', async () => {
    let received: unknown;
    server.use(
      http.post('*/api/v1/admin/ai/check', async ({ request }) => {
        received = await request.json();
        return new HttpResponse('event: delta\ndata: {"text":"ok"}\n\nevent: done\ndata: {}\n\n', {
          headers: { 'Content-Type': 'text/event-stream' },
        });
      }),
    );
    const events: ServerEvent[] = [];
    await postEventStream('/admin/ai/check', { feature: 'mentor' }, (event) => events.push(event));
    expect(received).toEqual({ feature: 'mentor' });
    expect(events.map((event) => event.event)).toEqual(['delta', 'done']);
  });

  it('throws the problem of a refused request', async () => {
    server.use(
      http.post('*/api/v1/admin/ai/check', () =>
        HttpResponse.json(
          {
            type: 'about:blank',
            title: 'Too Many Requests',
            status: 429,
            code: 'ai.limit_reached',
          },
          { status: 429 },
        ),
      ),
    );
    const error = await postEventStream('/admin/ai/check', {}, () => undefined).catch(
      (e: unknown) => e,
    );
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe('ai.limit_reached');
  });
});
