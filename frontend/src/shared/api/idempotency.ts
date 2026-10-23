import { useState } from 'react';

import { ApiError, NetworkError } from './client';

/**
 * True when the server may not have finished the request, so sending it
 * again must carry the same key: the answer was lost on the way, the server
 * failed (it frees the key on 5xx) or it is still working on the first one.
 */
function unsettled(error: unknown): boolean {
  if (error instanceof NetworkError) return true;
  if (error instanceof ApiError)
    return error.status >= 500 || error.code === 'idempotency.in_progress';
  return false;
}

/**
 * The `Idempotency-Key` of one logical submission of a form. Retrying the
 * same payload after a lost answer reuses the key, so the server replays the
 * first outcome instead of creating a duplicate; a changed payload or a
 * submission with a known outcome gets a new key.
 */
export class SubmissionKey {
  private current: { key: string; payload: string } | undefined;
  private readonly generate: () => string;

  constructor(generate: () => string = () => crypto.randomUUID()) {
    this.generate = generate;
  }

  /** The key to send with this payload. */
  for(payload: unknown): string {
    const serialized = JSON.stringify(payload ?? null);
    if (this.current?.payload !== serialized) {
      this.current = { key: this.generate(), payload: serialized };
    }
    return this.current.key;
  }

  /** Records the outcome: success or a definite refusal ends the submission. */
  settle(error: unknown): void {
    if (error === null || error === undefined || !unsettled(error)) this.current = undefined;
  }
}

/** One submission key per mounted form (mutation hook). */
export function useIdempotencyKey(): SubmissionKey {
  const [key] = useState(() => new SubmissionKey());
  return key;
}
