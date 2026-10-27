import { useCallback, useEffect, useRef, useState } from 'react';

import { ApiError, type Problem } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { postEventStream, type ServerEvent } from '@/shared/api/sse';
import type { Locale } from '@/shared/i18n';

import type { AiCheckResult, AiFeature } from './api';

export type CheckState =
  | { status: 'idle' }
  | { status: 'running'; feature: AiFeature; text: string }
  | { status: 'done'; feature: AiFeature; text: string; result: AiCheckResult }
  | { status: 'failed'; feature: AiFeature; text: string; message: string }
  | { status: 'stopped'; feature: AiFeature; text: string };

/** The JSON payload of an event; the server's events always carry objects. */
function payload(event: ServerEvent): unknown {
  try {
    return JSON.parse(event.data);
  } catch {
    return undefined;
  }
}

/**
 * Runs the connection check of one feature at a time, streaming the model's
 * answer as it is written. Leaving the page or starting another check stops it.
 */
export function useAiCheck() {
  const [state, setState] = useState<CheckState>({ status: 'idle' });
  const controller = useRef<AbortController | null>(null);

  useEffect(
    () => () => {
      controller.current?.abort();
    },
    [],
  );

  const start = useCallback((feature: AiFeature, locale: Locale) => {
    controller.current?.abort();
    const current = new AbortController();
    controller.current = current;
    let text = '';
    let finished = false;
    setState({ status: 'running', feature, text });

    const onEvent = (event: ServerEvent) => {
      if (event.event === 'delta') {
        text += (payload(event) as { text?: string } | undefined)?.text ?? '';
        setState({ status: 'running', feature, text });
      } else if (event.event === 'done') {
        const result = payload(event) as AiCheckResult | undefined;
        if (!result) return;
        finished = true;
        setState({ status: 'done', feature, text, result });
      } else if (event.event === 'error') {
        const problem = payload(event) as Problem | undefined;
        finished = true;
        setState({
          status: 'failed',
          feature,
          text,
          message: errorMessage(problem ? new ApiError(problem) : undefined),
        });
      }
    };

    postEventStream('/admin/ai/check', { feature, locale }, onEvent, current.signal)
      .then(() => {
        // The stream ended without a result: the connection broke midway.
        if (!finished && !current.signal.aborted) {
          setState({ status: 'failed', feature, text, message: errorMessage(undefined) });
        }
      })
      .catch((err: unknown) => {
        if (current.signal.aborted) return;
        setState({ status: 'failed', feature, text, message: errorMessage(err) });
      });
  }, []);

  const stop = useCallback(() => {
    const current = controller.current;
    if (!current) return;
    current.abort();
    controller.current = null;
    setState((prev) =>
      prev.status === 'running'
        ? { status: 'stopped', feature: prev.feature, text: prev.text }
        : prev,
    );
  }, []);

  const reset = useCallback(() => {
    controller.current?.abort();
    controller.current = null;
    setState({ status: 'idle' });
  }, []);

  return { state, start, stop, reset };
}
