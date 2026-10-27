import { useQueries, useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Check, ExternalLink, X } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  ConfirmDialog,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Field,
  SkeletonCard,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
  TruncatedText,
} from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import {
  showcaseReviewsQuery,
  usePublishShowcaseEntry,
  useReviewShowcaseEntry,
  type ShowcaseEntry,
  type ShowcaseStatus,
} from './api';
import { CardCover } from './card';
import { excerpt } from './logic';

const TABS: ShowcaseStatus[] = ['in_review', 'published', 'rejected', 'draft'];
const COMMENT_MAX = 1000;

/** Innovation managers review public pages before everyone can see them. */
export function ModerationPage() {
  const { t } = useTranslation('showcase');
  const [tab, setTab] = useState<ShowcaseStatus>('in_review');
  const counts = useQueries({ queries: TABS.map((status) => showcaseReviewsQuery(status)) });
  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-bold text-ink">{t('moderation.title')}</h1>
        <p className="text-fg-muted">{t('moderation.subtitle')}</p>
      </header>
      <Tabs
        value={tab}
        onValueChange={(value) => {
          setTab(value as ShowcaseStatus);
        }}
      >
        <TabsList>
          {TABS.map((status, i) => (
            <TabsTrigger key={status} value={status} count={counts[i]?.data?.length}>
              {t(`moderation.tabs.${status}`)}
            </TabsTrigger>
          ))}
        </TabsList>
        {TABS.map((status) => (
          <TabsContent key={status} value={status}>
            <ReviewList status={status} />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}

function ReviewList({ status }: { status: ShowcaseStatus }) {
  const { t } = useTranslation('showcase');
  const list = useQuery(showcaseReviewsQuery(status));
  if (list.isPending) {
    return (
      <div className="flex flex-col gap-4" aria-busy>
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }
  if (list.isError) return <Callout tone="danger">{errorMessage(list.error)}</Callout>;
  if (list.data.length === 0) {
    return (
      <EmptyState
        illustration="inbox"
        title={
          status === 'in_review' ? t('moderation.empty.in_review') : t('moderation.empty.other')
        }
        description={
          status === 'in_review'
            ? t('moderation.empty.in_reviewText')
            : t('moderation.empty.otherText')
        }
      />
    );
  }
  return (
    <ul className="flex flex-col gap-4">
      {list.data.map((entry) => (
        <li key={entry.project_id}>
          <ReviewCard entry={entry} />
        </li>
      ))}
    </ul>
  );
}

/** The team changed the page after the moderator opened it: they read it again. */
function reviewError(err: unknown, stale: string): string {
  return err instanceof ApiError && err.status === 412 ? stale : errorMessage(err);
}

function ReviewCard({ entry }: { entry: ShowcaseEntry }) {
  const { t } = useTranslation(['showcase', 'common']);
  const { relative } = useFormatters();
  const review = useReviewShowcaseEntry();
  const takeDown = usePublishShowcaseEntry(entry.project_id);
  const [rejecting, setRejecting] = useState(false);
  const [removing, setRemoving] = useState(false);
  // The decision is about the version on the screen.
  const approve = () => {
    review.mutate(
      { projectId: entry.project_id, version: entry.version, approve: true },
      {
        onSuccess: () => toast.success(t('moderation.approved')),
        onError: (err) => toast.error(reviewError(err, t('moderation.stale'))),
      },
    );
  };
  return (
    <article className="grid grid-cols-1 overflow-hidden rounded-xl border border-border bg-surface shadow-sm sm:grid-cols-[14rem_1fr]">
      <div className="aspect-video bg-surface-sunken sm:aspect-auto sm:h-full">
        <CardCover title={entry.title} industry={entry.industry} coverUrl={entry.cover_url} />
      </div>
      <div className="flex min-w-0 flex-col gap-3 p-5">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="primary" size="sm">
            {t(`industries.${entry.industry}`)}
          </Badge>
          <Badge size="sm">{t(`visibility.${entry.visibility}`)}</Badge>
          {entry.submitted_at && entry.status === 'in_review' && (
            <span className="text-xs text-fg-subtle">
              {t('moderation.submitted', { time: relative(entry.submitted_at) })}
            </span>
          )}
          {entry.published_at && entry.status === 'published' && (
            <span className="text-xs text-fg-subtle">
              {t('moderation.published', { time: relative(entry.published_at) })}
            </span>
          )}
        </div>
        <div className="flex flex-col gap-1">
          <h2 className="font-display text-lg font-bold text-ink">{entry.title}</h2>
          {entry.tagline && (
            <TruncatedText as="p" lines={2} className="text-sm text-fg-muted">
              {entry.tagline}
            </TruncatedText>
          )}
        </div>
        <dl className="grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
          {entry.sections.problem && (
            <div className="flex flex-col gap-0.5">
              <dt className="text-xs font-medium text-fg-subtle">{t('quick.problem')}</dt>
              <dd className="text-fg">{excerpt(entry.sections.problem, 220)}</dd>
            </div>
          )}
          {entry.sections.solution && (
            <div className="flex flex-col gap-0.5">
              <dt className="text-xs font-medium text-fg-subtle">{t('quick.solution')}</dt>
              <dd className="text-fg">{excerpt(entry.sections.solution, 220)}</dd>
            </div>
          )}
        </dl>
        {entry.team.length > 0 && (
          <p className="text-xs text-fg-muted">
            {t('project.sections.team')}: {entry.team.map((m) => m.name).join(', ')}
          </p>
        )}
        {entry.status === 'rejected' && entry.review_comment && (
          <Callout tone="danger" title={t('editor.state.comment')}>
            <span className="whitespace-pre-line">{entry.review_comment}</span>
          </Callout>
        )}
        {entry.held && <Callout tone="warning">{t('moderation.held')}</Callout>}
        <div className="mt-auto flex flex-wrap items-center gap-2 pt-1">
          {entry.status === 'in_review' && (
            <Button size="sm" leftIcon={<Check />} loading={review.isPending} onClick={approve}>
              {t('moderation.approve')}
            </Button>
          )}
          {/* A published public page can be taken off the public showcase too. */}
          {(entry.status === 'in_review' ||
            (entry.status === 'published' && entry.visibility === 'public')) && (
            <>
              <Button
                size="sm"
                variant="secondary"
                leftIcon={<X />}
                disabled={review.isPending}
                onClick={() => {
                  setRejecting(true);
                }}
              >
                {t('moderation.reject')}
              </Button>
            </>
          )}
          {/* A take-down hides the page everywhere until a moderator approves it again. */}
          {entry.published_at && (
            <Button
              size="sm"
              variant="ghost"
              disabled={takeDown.isPending}
              onClick={() => {
                setRemoving(true);
              }}
            >
              {t('moderation.takeDown')}
            </Button>
          )}
          <Button asChild size="sm" variant="ghost">
            <Link to="/app/projects/$projectId/showcase" params={{ projectId: entry.project_id }}>
              {t('moderation.openProject')}
            </Link>
          </Button>
          {entry.url && entry.published_at && (
            <Button asChild size="sm" variant="ghost">
              <a href={entry.url} target="_blank" rel="noopener">
                {t('moderation.open')}
                <ExternalLink aria-hidden />
              </a>
            </Button>
          )}
        </div>
      </div>
      <RejectDialog
        open={rejecting}
        onOpenChange={setRejecting}
        title={entry.title}
        loading={review.isPending}
        onReject={(comment) => {
          review.mutate(
            { projectId: entry.project_id, version: entry.version, approve: false, comment },
            {
              onSuccess: () => {
                setRejecting(false);
                toast.success(t('moderation.rejected'));
              },
              onError: (err) => toast.error(reviewError(err, t('moderation.stale'))),
            },
          );
        }}
      />
      <ConfirmDialog
        open={removing}
        onOpenChange={setRemoving}
        title={t('moderation.takeDownDialog.title', { title: entry.title })}
        description={t('moderation.takeDownDialog.text')}
        confirmLabel={t('moderation.takeDown')}
        tone="danger"
        loading={takeDown.isPending}
        onConfirm={() => {
          takeDown.mutate(false, {
            onSuccess: () => {
              setRemoving(false);
              toast.success(t('moderation.takenDown'));
            },
            onError: (err) => toast.error(errorMessage(err)),
          });
        }}
      />
    </article>
  );
}

function RejectDialog({
  open,
  onOpenChange,
  title,
  loading,
  onReject,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  loading: boolean;
  onReject: (comment: string) => void;
}) {
  const { t } = useTranslation(['showcase', 'common']);
  const [comment, setComment] = useState('');
  const [error, setError] = useState<string>();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form
          noValidate
          className="flex min-h-0 flex-col"
          onSubmit={(event) => {
            event.preventDefault();
            if (!comment.trim()) {
              setError(t('common:validation.required'));
              return;
            }
            onReject(comment.trim());
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('moderation.rejectDialog.title', { title })}</DialogTitle>
            <DialogDescription>{t('moderation.rejectDialog.text')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Field
              label={t('moderation.rejectDialog.comment')}
              required
              error={error}
              counter={{ value: comment.length, max: COMMENT_MAX }}
            >
              <Textarea
                rows={4}
                value={comment}
                maxLength={COMMENT_MAX}
                onChange={(e) => {
                  setComment(e.target.value);
                  setError(undefined);
                }}
              />
            </Field>
          </DialogBody>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                onOpenChange(false);
              }}
            >
              {t('common:actions.cancel')}
            </Button>
            <Button type="submit" variant="danger" loading={loading}>
              {t('moderation.reject')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
