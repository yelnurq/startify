import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react';

import { reportPresence, type Presence } from '@/features/projects/api';
import { ApiError, type Problem } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { postEventStream, type ServerEvent } from '@/shared/api/sse';

import {
  fetchStep,
  mentorQuery,
  saveStep,
  stepQuery,
  stepsQuery,
  type MentorConversation,
  type MentorMessage,
  type MentorMode,
} from './api';
import { StepEditor, type Tagged } from './step-editor';

/** One editor per mounted step form; it keeps edits across re-renders. */
export function useStepEditor(projectId: string, stepKey: string, initial: Tagged) {
  const queryClient = useQueryClient();
  const [editor] = useState(
    () =>
      new StepEditor(
        initial,
        {
          save: (body, etag) => saveStep(projectId, stepKey, body, etag),
          load: () => fetchStep(projectId, stepKey),
        },
        1200,
        () => {
          void queryClient.invalidateQueries({ queryKey: stepsQuery(projectId).queryKey });
          void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'card'] });
          void queryClient.invalidateQueries({ queryKey: ['projects', projectId, 'activity'] });
        },
      ),
  );
  const snapshot = useSyncExternalStore(editor.subscribe, editor.getSnapshot);

  useEffect(() => {
    editor.resume();
    // Leaving the step sends what is left; the cache keeps the last version.
    return () => {
      editor.dispose();
      queryClient.setQueryData(stepQuery(projectId, stepKey).queryKey, {
        data: { ...editor.getSnapshot().step, values: editor.getSnapshot().values },
        etag: editor.currentEtag,
      });
    };
  }, [editor, queryClient, projectId, stepKey]);

  useEffect(() => {
    const online = () => void editor.save();
    window.addEventListener('online', online);
    return () => {
      window.removeEventListener('online', online);
    };
  }, [editor]);

  return { editor, snapshot };
}

const PRESENCE_INTERVAL = 15_000;

/**
 * Reports presence every 15 seconds while the page is visible and returns
 * who else is in the project and the current step versions.
 */
export function usePresence(projectId: string, stepKey: string | undefined) {
  const [presence, setPresence] = useState<Presence | null>(null);
  const step = useRef(stepKey);
  useEffect(() => {
    step.current = stepKey;
  }, [stepKey]);

  useEffect(() => {
    let stopped = false;
    const report = () => {
      if (document.visibilityState !== 'visible') return;
      reportPresence(projectId, step.current)
        .then((value) => {
          if (!stopped) setPresence(value);
        })
        .catch(() => undefined);
    };
    report();
    const timer = setInterval(report, PRESENCE_INTERVAL);
    document.addEventListener('visibilitychange', report);
    return () => {
      stopped = true;
      clearInterval(timer);
      document.removeEventListener('visibilitychange', report);
    };
  }, [projectId]);

  // A new step is announced at once.
  useEffect(() => {
    if (!stepKey) return;
    reportPresence(projectId, stepKey)
      .then(setPresence)
      .catch(() => undefined);
  }, [projectId, stepKey]);

  return presence;
}

export type MentorStream =
  | { status: 'idle' }
  | { status: 'streaming'; text: string }
  | { status: 'failed'; text: string; message: string };

function payload(event: ServerEvent): unknown {
  try {
    return JSON.parse(event.data);
  } catch {
    return undefined;
  }
}

/** Sends a question to the mentor and streams the answer into the conversation. */
export function useMentor(projectId: string, stepKey: string) {
  const queryClient = useQueryClient();
  const [stream, setStream] = useState<MentorStream>({ status: 'idle' });
  const controller = useRef<AbortController | null>(null);
  const key = mentorQuery(projectId, stepKey).queryKey;

  useEffect(
    () => () => {
      controller.current?.abort();
    },
    [],
  );

  const append = useCallback(
    (message: MentorMessage) => {
      queryClient.setQueryData(key, (data) => {
        if (!data) return data;
        const [first, ...rest] = data.pages;
        if (!first) return data;
        const page: MentorConversation = {
          ...first,
          items: [...first.items.filter((m) => m.id !== message.id), message],
        };
        return { ...data, pages: [page, ...rest] };
      });
    },
    [queryClient, key],
  );

  const send = useCallback(
    (content: string, mode: MentorMode | undefined) => {
      controller.current?.abort();
      const current = new AbortController();
      controller.current = current;
      let text = '';
      let finished = false;
      setStream({ status: 'streaming', text });

      const onEvent = (event: ServerEvent) => {
        if (event.event === 'message') {
          const message = payload(event) as MentorMessage | undefined;
          if (message) append(message);
        } else if (event.event === 'delta') {
          text += (payload(event) as { text?: string } | undefined)?.text ?? '';
          setStream({ status: 'streaming', text });
        } else if (event.event === 'done') {
          const message = payload(event) as MentorMessage | undefined;
          finished = true;
          if (message) append(message);
          setStream({ status: 'idle' });
        } else if (event.event === 'error') {
          const problem = payload(event) as Problem | undefined;
          finished = true;
          setStream({
            status: 'failed',
            text,
            message: errorMessage(problem ? new ApiError(problem) : undefined),
          });
        }
      };

      postEventStream(
        `/projects/${projectId}/steps/${stepKey}/mentor`,
        mode ? { content, mode } : { content },
        onEvent,
        current.signal,
      )
        .then(() => {
          if (!finished && !current.signal.aborted) {
            setStream({ status: 'failed', text, message: errorMessage(undefined) });
          }
        })
        .catch((err: unknown) => {
          if (current.signal.aborted) return;
          setStream({ status: 'failed', text, message: errorMessage(err) });
        })
        .finally(() => {
          if (controller.current === current) controller.current = null;
        });
    },
    [projectId, stepKey, append],
  );

  /** Stops the answer; the server keeps the part written so far. */
  const stop = useCallback(() => {
    controller.current?.abort();
    controller.current = null;
    setStream({ status: 'idle' });
    // The saved partial answer replaces the one on screen.
    setTimeout(() => void queryClient.invalidateQueries({ queryKey: key }), 500);
  }, [queryClient, key]);

  const dismiss = useCallback(() => {
    setStream({ status: 'idle' });
  }, []);

  return { stream, send, stop, dismiss };
}
