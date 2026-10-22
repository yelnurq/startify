import { apiFetch, NetworkError, throwProblem } from './client';

/** One server-sent event. */
export interface ServerEvent {
  event: string;
  data: string;
}

/**
 * Incremental text/event-stream parser (WHATWG HTML, "Server-sent events"):
 * feed it decoded text as it arrives and it returns the complete events.
 */
export class EventStreamParser {
  private buffer = '';
  private type = '';
  private data: string[] = [];

  push(chunk: string): ServerEvent[] {
    this.buffer += chunk;
    const events: ServerEvent[] = [];
    for (;;) {
      const match = /\r\n|\r|\n/.exec(this.buffer);
      // A trailing CR may be the first half of CRLF: wait for the next chunk.
      if (!match || (match[0] === '\r' && match.index === this.buffer.length - 1)) break;
      const line = this.buffer.slice(0, match.index);
      this.buffer = this.buffer.slice(match.index + match[0].length);
      const event = this.line(line);
      if (event) events.push(event);
    }
    return events;
  }

  /** Flushes the stream end: an unterminated last event is discarded, as the spec says. */
  end(): ServerEvent[] {
    this.buffer = '';
    this.type = '';
    this.data = [];
    return [];
  }

  private line(line: string): ServerEvent | undefined {
    if (line === '') {
      const event =
        this.data.length > 0
          ? { event: this.type || 'message', data: this.data.join('\n') }
          : undefined;
      this.type = '';
      this.data = [];
      return event;
    }
    if (line.startsWith(':')) return undefined;
    const colon = line.indexOf(':');
    const field = colon < 0 ? line : line.slice(0, colon);
    let value = colon < 0 ? '' : line.slice(colon + 1);
    if (value.startsWith(' ')) value = value.slice(1);
    if (field === 'event') this.type = value;
    else if (field === 'data') this.data.push(value);
    return undefined;
  }
}

/** Reads a response body as an event stream until it ends or the signal aborts. */
export async function readEventStream(
  response: Response,
  onEvent: (event: ServerEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  if (!response.body) return;
  const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
  const parser = new EventStreamParser();
  const cancel = () => void reader.cancel().catch(() => undefined);
  signal?.addEventListener('abort', cancel, { once: true });
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      for (const event of parser.push(value)) onEvent(event);
    }
    parser.end();
  } finally {
    signal?.removeEventListener('abort', cancel);
    reader.releaseLock();
  }
}

/**
 * POSTs JSON to an API path that answers with an event stream. Failures before
 * the stream starts throw ApiError (or NetworkError); aborting stops reading.
 */
export async function postEventStream(
  path: string,
  body: unknown,
  onEvent: (event: ServerEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  let response: Response;
  try {
    response = await apiFetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      body: JSON.stringify(body),
      signal: signal ?? null,
    });
  } catch (err) {
    if (signal?.aborted) throw err;
    throw new NetworkError(err);
  }
  if (!response.ok) await throwProblem(response);
  try {
    await readEventStream(response, onEvent, signal);
  } catch (err) {
    if (signal?.aborted) throw err;
    throw new NetworkError(err);
  }
}
