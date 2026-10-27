import { useQuery } from '@tanstack/react-query';
import { Check, CornerDownRight, MessageSquare, Pencil, RotateCcw, Trash2 } from 'lucide-react';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Callout,
  cn,
  ConfirmDialog,
  EmptyState,
  IconButton,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SkeletonText,
  Textarea,
  toast,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import {
  commentsQuery,
  INCUBATOR_LIMITS,
  useCreateComment,
  useDeleteComment,
  useUpdateComment,
  type Comment,
  type TemplateStep,
} from './api';
import { mentionedIds, mentionQuery, type Mentionable } from './mentions';

function MentionTextarea({
  value,
  onChange,
  people,
  label,
  placeholder,
  autoFocus,
  onSubmit,
}: {
  value: string;
  onChange: (value: string) => void;
  people: Mentionable[];
  label: string;
  placeholder: string;
  autoFocus?: boolean;
  onSubmit: () => void;
}) {
  const { t } = useTranslation('incubator');
  const ref = useRef<HTMLTextAreaElement>(null);
  const [caret, setCaret] = useState(0);
  const query = mentionQuery(value.slice(0, caret));
  const matches =
    query === null
      ? []
      : people.filter((p) => p.name.toLowerCase().includes(query.toLowerCase())).slice(0, 5);

  const insert = (person: Mentionable) => {
    const before = value.slice(0, caret).replace(/@[\p{L}\p{N}.-]*$/u, `@${person.name} `);
    const next = before + value.slice(caret);
    onChange(next);
    requestAnimationFrame(() => {
      ref.current?.focus();
      ref.current?.setSelectionRange(before.length, before.length);
      setCaret(before.length);
    });
  };

  return (
    <div className="relative">
      <Textarea
        ref={ref}
        value={value}
        rows={2}
        maxLength={INCUBATOR_LIMITS.comment}
        aria-label={label}
        placeholder={placeholder}
        autoFocus={autoFocus}
        onChange={(e) => {
          onChange(e.target.value);
          setCaret(e.target.selectionStart);
        }}
        onSelect={(e) => {
          setCaret(e.currentTarget.selectionStart);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
            e.preventDefault();
            onSubmit();
          } else if (e.key === 'Enter' && matches[0] && !e.shiftKey) {
            e.preventDefault();
            insert(matches[0]);
          }
        }}
      />
      {matches.length > 0 && (
        <ul
          aria-label={t('comments.mention')}
          className="absolute inset-x-0 top-full z-10 mt-1 overflow-hidden rounded-xl border border-border bg-surface shadow-lg"
        >
          {matches.map((person) => (
            <li key={person.id}>
              <button
                type="button"
                className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-surface-raised focus-visible:bg-surface-raised"
                onMouseDown={(e) => {
                  e.preventDefault();
                }}
                onClick={() => {
                  insert(person);
                }}
              >
                <Avatar name={person.name} src={person.avatar} size="xs" />
                {person.name}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function Composer({
  people,
  label,
  placeholder,
  submitLabel,
  initial = '',
  autoFocus,
  loading,
  onSubmit,
  onCancel,
}: {
  people: Mentionable[];
  label: string;
  placeholder: string;
  submitLabel: string;
  initial?: string;
  autoFocus?: boolean;
  loading: boolean;
  onSubmit: (body: string, mentions: string[]) => Promise<unknown>;
  onCancel?: () => void;
}) {
  const { t } = useTranslation(['incubator', 'common']);
  const [body, setBody] = useState(initial);
  const submit = () => {
    const text = body.trim();
    if (!text) return;
    onSubmit(text, mentionedIds(text, people).slice(0, INCUBATOR_LIMITS.mentions))
      .then(() => {
        setBody('');
      })
      .catch((err: unknown) => {
        toast.error(errorMessage(err));
      });
  };
  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      <MentionTextarea
        value={body}
        onChange={setBody}
        people={people}
        label={label}
        placeholder={placeholder}
        autoFocus={autoFocus}
        onSubmit={submit}
      />
      <div className="flex flex-wrap items-center justify-end gap-2">
        <span className="mr-auto text-xs text-fg-subtle">{t('comments.hint')}</span>
        {onCancel && (
          <Button size="sm" variant="ghost" onClick={onCancel}>
            {t('common:actions.cancel')}
          </Button>
        )}
        <Button size="sm" type="submit" loading={loading} disabled={!body.trim()}>
          {submitLabel}
        </Button>
      </div>
    </form>
  );
}

interface CommentsSheetProps {
  projectId: string;
  step: TemplateStep;
  /** A field key, or null for the whole step; undefined closes the sheet. */
  field: string | null | undefined;
  people: Mentionable[];
  canComment: boolean;
  canModerate: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Threads of comments on a step or one of its fields. */
export function CommentsSheet({
  projectId,
  step,
  field,
  people,
  canComment,
  canModerate,
  onOpenChange,
}: CommentsSheetProps) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const open = field !== undefined;
  const query = useQuery({ ...commentsQuery(projectId, step.key), enabled: open });
  const create = useCreateComment(projectId, step.key);
  const fieldDef = field ? step.fields.find((f) => f.key === field) : undefined;
  const label = fieldDef ? text(fieldDef.label) : text(step.title);

  const threads = useMemo(() => {
    const all = query.data ?? [];
    const roots = all.filter((c) => !c.parent_id && (field === null || c.field_key === field));
    return roots.map((root) => ({
      root,
      replies: all.filter((c) => c.parent_id === root.id),
    }));
  }, [query.data, field]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="md">
        <div className="flex flex-col gap-1 border-b border-border px-5 pt-5 pr-14 pb-4">
          <SheetTitle className="flex items-center gap-2 text-base font-semibold text-fg">
            <MessageSquare className="size-4 text-fg-subtle" aria-hidden />
            {t('comments.title')}
          </SheetTitle>
          <SheetDescription className="text-sm text-fg-muted">{label}</SheetDescription>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-4">
          {query.isPending && <SkeletonText lines={5} />}
          {query.isError && <Callout tone="danger">{errorMessage(query.error)}</Callout>}
          {query.isSuccess && threads.length === 0 && (
            <EmptyState
              size="sm"
              illustration="inbox"
              title={t('comments.empty')}
              description={canComment ? t('comments.emptyText') : undefined}
            />
          )}
          {threads.map(({ root, replies }) => (
            <Thread
              key={root.id}
              projectId={projectId}
              step={step}
              root={root}
              replies={replies}
              people={people}
              showField={field === null}
              canComment={canComment}
              canModerate={canModerate}
            />
          ))}
        </div>
        {canComment && (
          <div className="border-t border-border p-4">
            <Composer
              people={people}
              label={t('comments.new')}
              placeholder={t('comments.placeholder')}
              submitLabel={t('comments.send')}
              loading={create.isPending}
              onSubmit={(body, mentions) =>
                create.mutateAsync({
                  step_key: step.key,
                  ...(field ? { field_key: field } : {}),
                  body,
                  mentions,
                })
              }
            />
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}

function Thread({
  projectId,
  step,
  root,
  replies,
  people,
  showField,
  canComment,
  canModerate,
}: {
  projectId: string;
  step: TemplateStep;
  root: Comment;
  replies: Comment[];
  people: Mentionable[];
  showField: boolean;
  canComment: boolean;
  canModerate: boolean;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const [replying, setReplying] = useState(false);
  const create = useCreateComment(projectId, step.key);
  const update = useUpdateComment(projectId, step.key);
  const field = root.field_key ? step.fields.find((f) => f.key === root.field_key) : undefined;
  return (
    <article
      className={cn(
        'flex flex-col gap-3 rounded-xl border border-border p-4',
        root.resolved && 'bg-surface-raised',
      )}
    >
      {(showField || root.resolved) && (
        <div className="flex flex-wrap items-center gap-2">
          {showField && (
            <Badge size="sm" tone="neutral">
              {field ? text(field.label) : t('comments.wholeStep')}
            </Badge>
          )}
          {root.resolved && (
            <Badge size="sm" tone="success" icon={<Check />}>
              {t('comments.resolved')}
            </Badge>
          )}
        </div>
      )}
      <CommentItem
        projectId={projectId}
        stepKey={step.key}
        comment={root}
        people={people}
        canModerate={canModerate}
      />
      {replies.map((reply) => (
        <div key={reply.id} className="flex gap-2 pl-2">
          <CornerDownRight className="mt-1 size-4 shrink-0 text-fg-faint" aria-hidden />
          <CommentItem
            projectId={projectId}
            stepKey={step.key}
            comment={reply}
            people={people}
            canModerate={canModerate}
          />
        </div>
      ))}
      {canComment && (
        <div className="flex flex-wrap gap-2">
          {!replying && (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setReplying(true);
              }}
            >
              {t('comments.reply')}
            </Button>
          )}
          <Button
            size="sm"
            variant="ghost"
            leftIcon={root.resolved ? <RotateCcw /> : <Check />}
            loading={update.isPending}
            onClick={() => {
              update.mutate(
                { commentId: root.id, body: { resolved: !root.resolved } },
                {
                  onError: (err) => {
                    toast.error(errorMessage(err));
                  },
                },
              );
            }}
          >
            {root.resolved ? t('comments.reopen') : t('comments.resolve')}
          </Button>
        </div>
      )}
      {replying && (
        <Composer
          people={people}
          label={t('comments.replyLabel')}
          placeholder={t('comments.replyPlaceholder')}
          submitLabel={t('comments.reply')}
          autoFocus
          loading={create.isPending}
          onCancel={() => {
            setReplying(false);
          }}
          onSubmit={async (body, mentions) => {
            await create.mutateAsync({
              step_key: step.key,
              ...(root.field_key ? { field_key: root.field_key } : {}),
              parent_id: root.id,
              body,
              mentions,
            });
            setReplying(false);
          }}
        />
      )}
    </article>
  );
}

function CommentItem({
  projectId,
  stepKey,
  comment,
  people,
  canModerate,
}: {
  projectId: string;
  stepKey: string;
  comment: Comment;
  people: Mentionable[];
  canModerate: boolean;
}) {
  const { t } = useTranslation(['incubator', 'common']);
  const format = useFormatters();
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const update = useUpdateComment(projectId, stepKey);
  const remove = useDeleteComment(projectId, stepKey);
  const name = comment.author?.full_name ?? t('comments.unknown');
  const edited = comment.updated_at !== comment.created_at;
  return (
    <div className="flex min-w-0 flex-1 gap-2.5">
      <Avatar name={name} src={comment.author?.avatar_url} size="sm" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="text-sm font-medium text-fg">{name}</span>
          <span className="text-xs text-fg-subtle">
            {format.relative(comment.created_at)}
            {edited && ` · ${t('comments.edited')}`}
          </span>
          <span className="ml-auto flex">
            {comment.editable && !editing && (
              <IconButton
                size="sm"
                label={t('comments.edit')}
                icon={<Pencil />}
                onClick={() => {
                  setEditing(true);
                }}
              />
            )}
            {(comment.editable || canModerate) && (
              <IconButton
                size="sm"
                label={t('comments.delete')}
                icon={<Trash2 />}
                onClick={() => {
                  setConfirm(true);
                }}
              />
            )}
          </span>
        </div>
        {editing ? (
          <Composer
            people={people}
            label={t('comments.edit')}
            placeholder={t('comments.placeholder')}
            submitLabel={t('common:actions.save')}
            initial={comment.body}
            autoFocus
            loading={update.isPending}
            onCancel={() => {
              setEditing(false);
            }}
            onSubmit={async (body) => {
              await update.mutateAsync({ commentId: comment.id, body: { body } });
              setEditing(false);
            }}
          />
        ) : (
          <p className="text-sm leading-relaxed break-words whitespace-pre-wrap text-fg">
            {comment.body}
          </p>
        )}
      </div>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={t('comments.deleteTitle')}
        description={t('comments.deleteText')}
        confirmLabel={t('comments.delete')}
        tone="danger"
        loading={remove.isPending}
        onConfirm={() => {
          remove.mutate(comment.id, {
            onSuccess: () => {
              setConfirm(false);
            },
            onError: (err) => {
              toast.error(errorMessage(err));
            },
          });
        }}
      />
    </div>
  );
}
