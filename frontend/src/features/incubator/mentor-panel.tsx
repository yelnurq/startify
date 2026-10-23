import { useInfiniteQuery } from '@tanstack/react-query';
import { Bot, BookOpenText, Check, SendHorizontal, Sparkles, Square } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Button,
  Callout,
  ChatBubble,
  cn,
  IconButton,
  SegmentedControl,
  SkeletonText,
  Tag,
  Textarea,
  toast,
  TruncatedText,
} from '@/design-system';
import { AiNotice } from '@/features/ai/ai-notice';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import {
  INCUBATOR_LIMITS,
  mentorQuery,
  type MentorMessage,
  type MentorMode,
  type TemplateStep,
} from './api';
import { useMentor } from './hooks';
import { LightMarkdown } from './light-markdown';
import { parseMentorContent, suggestionValue } from './suggestions';
import { suggestible } from './template';
import type { FieldValue } from './values';

interface MentorPanelProps {
  projectId: string;
  step: TemplateStep;
  /** Writes a suggestion into a field; null when the form is read-only. */
  onApply: ((field: string, value: FieldValue) => Promise<void>) | null;
}

function MentorAvatar() {
  return (
    <span className="grid size-7 place-items-center rounded-full bg-primary-soft text-primary-soft-fg">
      <Bot className="size-4" aria-hidden />
    </span>
  );
}

/** The AI mentor of a step: conversation, streaming answers, suggestions for fields. */
export function MentorPanel({ projectId, step, onApply }: MentorPanelProps) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const query = useInfiniteQuery(mentorQuery(projectId, step.key));
  const { stream, send, stop, dismiss } = useMentor(projectId, step.key);
  const [draft, setDraft] = useState('');
  const [mode, setMode] = useState<MentorMode | undefined>(undefined);
  const scroller = useRef<HTMLDivElement>(null);

  const first = query.data?.pages[0];
  const modes = first?.modes ?? [];
  const activeMode = mode ?? modes[0];
  // Pages go back in time; within a page messages are oldest first.
  const messages = useMemo(
    () => [...(query.data?.pages ?? [])].reverse().flatMap((page) => page.items),
    [query.data],
  );
  const streaming = stream.status === 'streaming';

  useEffect(() => {
    const el = scroller.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages.length, stream]);

  const submit = () => {
    const content = draft.trim();
    if (!content || streaming) return;
    setDraft('');
    // A question the server never got (limit, no network) comes back into
    // the field, unless the person has started typing another one.
    send(content, modes.length > 1 ? activeMode : undefined, (unsent) => {
      setDraft((current) => current || unsent);
    });
  };

  return (
    <div className="flex h-full min-h-0 flex-col">
      {modes.length > 1 && activeMode && (
        <div className="flex items-center gap-2 border-b border-border px-4 py-2.5">
          <SegmentedControl
            size="sm"
            label={t('mentor.mode')}
            value={activeMode}
            onValueChange={setMode}
            options={modes.map((m) => ({ value: m, label: t(`mentor.modes.${m}`) }))}
          />
        </div>
      )}

      <div
        ref={scroller}
        className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-4"
        aria-live="polite"
        aria-busy={streaming}
      >
        <AiNotice className="text-xs" />
        {query.hasNextPage && (
          <Button
            variant="ghost"
            size="sm"
            className="self-center"
            loading={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            {t('mentor.earlier')}
          </Button>
        )}
        {query.isPending && <SkeletonText lines={4} />}
        {query.isError && <Callout tone="danger">{errorMessage(query.error)}</Callout>}
        {query.isSuccess && messages.length === 0 && !streaming && (
          <div className="flex flex-col items-start gap-3">
            <ChatBubble avatar={<MentorAvatar />}>
              <p>{t('mentor.welcome', { step: text(step.title) })}</p>
            </ChatBubble>
            {first?.can_ask && (
              <div className="flex flex-wrap gap-2 pl-9">
                {(['start', 'improve', 'check'] as const).map((prompt) => (
                  <Tag
                    key={prompt}
                    onToggle={() => {
                      send(
                        t(`mentor.prompts.${prompt}`),
                        modes.length > 1 ? activeMode : undefined,
                      );
                    }}
                  >
                    {t(`mentor.prompts.${prompt}`)}
                  </Tag>
                ))}
              </div>
            )}
          </div>
        )}
        {messages.map((message) => (
          <Message key={message.id} message={message} step={step} onApply={onApply} />
        ))}
        {stream.status !== 'idle' && (
          <ChatBubble avatar={<MentorAvatar />} muted={stream.status === 'failed'}>
            {stream.text ? (
              <MentorContent content={stream.text} streaming step={step} onApply={null} />
            ) : (
              <span className="flex items-center gap-2 text-fg-subtle">
                <Sparkles className="size-4 animate-pulse" aria-hidden />
                {t('mentor.writing')}
              </span>
            )}
            {stream.status === 'failed' && (
              <div className="flex flex-col items-start gap-2">
                <p className="text-sm text-danger">{stream.message}</p>
                <Button size="sm" variant="secondary" onClick={dismiss}>
                  {t('mentor.dismiss')}
                </Button>
              </div>
            )}
          </ChatBubble>
        )}
      </div>

      {first?.can_ask === false ? (
        <p className="border-t border-border p-4 text-sm text-fg-subtle">{t('mentor.readOnly')}</p>
      ) : (
        <form
          className="flex flex-col gap-1.5 border-t border-border p-3"
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <div className="flex items-end gap-2">
            <Textarea
              rows={1}
              value={draft}
              maxLength={INCUBATOR_LIMITS.mentor}
              className="max-h-40 min-h-10"
              placeholder={t('mentor.placeholder')}
              aria-label={t('mentor.placeholder')}
              disabled={query.isPending}
              onChange={(e) => {
                setDraft(e.target.value);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  submit();
                }
              }}
            />
            {streaming ? (
              <IconButton
                variant="secondary"
                label={t('mentor.stop')}
                icon={<Square />}
                onClick={stop}
              />
            ) : (
              <IconButton
                type="submit"
                variant="primary"
                label={t('mentor.send')}
                icon={<SendHorizontal />}
                disabled={!draft.trim()}
              />
            )}
          </div>
          <p className="text-xs text-fg-subtle">{t('mentor.hint')}</p>
        </form>
      )}
    </div>
  );
}

function Message({
  message,
  step,
  onApply,
}: {
  message: MentorMessage;
  step: TemplateStep;
  onApply: MentorPanelProps['onApply'];
}) {
  const { t } = useTranslation('incubator');
  const format = useFormatters();
  const own = message.role === 'user';
  const name = message.author?.full_name ?? t('mentor.you');
  return (
    <ChatBubble
      own={own}
      muted={message.status !== 'ok'}
      avatar={
        own ? <Avatar name={name} src={message.author?.avatar_url} size="xs" /> : <MentorAvatar />
      }
      meta={
        <>
          {own ? name : t('mentor.name')} ·{' '}
          {format.dateTime(message.created_at, { timeStyle: 'short' })}
        </>
      }
    >
      {own ? (
        <p className="whitespace-pre-wrap">{message.content}</p>
      ) : (
        <MentorContent content={message.content} streaming={false} step={step} onApply={onApply} />
      )}
      {message.status === 'cancelled' && (
        <p className="text-xs text-fg-subtle">{t('mentor.cancelled')}</p>
      )}
      {message.status === 'error' && <p className="text-xs text-danger">{t('mentor.failed')}</p>}
      {message.sources.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 border-t border-border pt-2">
          <span className="text-xs text-fg-subtle">{t('mentor.sources')}</span>
          {message.sources.map((source) => (
            <span
              key={source.document_id}
              className="inline-flex max-w-full items-center gap-1 rounded-full border border-border bg-surface px-2 py-0.5 text-xs text-fg-muted"
            >
              <BookOpenText className="size-3 shrink-0" aria-hidden />
              <TruncatedText>{source.title}</TruncatedText>
            </span>
          ))}
        </div>
      )}
    </ChatBubble>
  );
}

function MentorContent({
  content,
  streaming,
  step,
  onApply,
}: {
  content: string;
  streaming: boolean;
  step: TemplateStep;
  onApply: MentorPanelProps['onApply'];
}) {
  return (
    <>
      {parseMentorContent(content, streaming).map((segment, index) =>
        segment.kind === 'text' ? (
          <div key={index} className="flex flex-col gap-2">
            <LightMarkdown text={segment.text} />
          </div>
        ) : (
          <SuggestionCard
            key={index}
            field={segment.field}
            text={segment.text}
            complete={segment.complete}
            step={step}
            onApply={onApply}
          />
        ),
      )}
    </>
  );
}

function SuggestionCard({
  field,
  text: suggestion,
  complete,
  step,
  onApply,
}: {
  field: string;
  text: string;
  complete: boolean;
  step: TemplateStep;
  onApply: MentorPanelProps['onApply'];
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const [state, setState] = useState<'idle' | 'applying' | 'applied'>('idle');
  const def = step.fields.find((f) => f.key === field);
  const usable = def !== undefined && suggestible(def);
  const label = def ? text(def.label) : field;
  return (
    <div className="rounded-xl border border-primary-border bg-primary-soft p-3 text-sm">
      <p className="mb-1.5 flex items-center gap-1.5 text-xs font-semibold text-primary-soft-fg">
        <Sparkles className="size-3.5" aria-hidden />
        {t('mentor.suggestion', { field: label })}
      </p>
      <p className="whitespace-pre-wrap text-fg">{suggestion}</p>
      {complete && onApply && usable && (
        <Button
          size="sm"
          variant="soft"
          className={cn('mt-2.5', state === 'applied' && 'pointer-events-none')}
          leftIcon={<Check />}
          loading={state === 'applying'}
          onClick={() => {
            setState('applying');
            onApply(field, suggestionValue(suggestion, def.type))
              .then(() => {
                setState('applied');
                toast.success(t('mentor.applied', { field: label }));
              })
              .catch((err: unknown) => {
                setState('idle');
                toast.error(errorMessage(err));
              });
          }}
        >
          {state === 'applied' ? t('mentor.appliedShort') : t('mentor.apply')}
        </Button>
      )}
    </div>
  );
}
