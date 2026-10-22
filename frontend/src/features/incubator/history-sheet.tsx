import { useInfiniteQuery } from '@tanstack/react-query';
import { History, RotateCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Callout,
  EmptyState,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SkeletonText,
} from '@/design-system';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';

import { revisionsQuery, type Revision } from './api';
import { ValueDiff } from './step-panels';
import type { TemplateField } from './values';

const SOURCE_TONE = { edit: 'neutral', mentor: 'primary', restore: 'gold' } as const;

interface HistorySheetProps {
  projectId: string;
  stepKey: string;
  field: TemplateField | undefined;
  label: string;
  /** Null hides restoring (read-only access). */
  onRestore: ((revision: Revision) => Promise<void>) | null;
  restoring: string | null;
  onOpenChange: (open: boolean) => void;
}

/** Every saved value of a field with the words that changed each time. */
export function HistorySheet({
  projectId,
  stepKey,
  field,
  label,
  onRestore,
  restoring,
  onOpenChange,
}: HistorySheetProps) {
  const { t } = useTranslation('incubator');
  const format = useFormatters();
  const query = useInfiniteQuery({
    ...revisionsQuery(projectId, stepKey, field?.key ?? ''),
    enabled: Boolean(field),
  });
  const revisions = query.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <Sheet open={Boolean(field)} onOpenChange={onOpenChange}>
      <SheetContent size="lg">
        <div className="flex flex-col gap-1 border-b border-border px-5 pt-5 pr-14 pb-4">
          <SheetTitle className="flex items-center gap-2 text-base font-semibold text-fg">
            <History className="size-4 text-fg-subtle" aria-hidden />
            {t('history.title')}
          </SheetTitle>
          <SheetDescription className="text-sm text-fg-muted">{label}</SheetDescription>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {query.isPending && <SkeletonText lines={6} />}
          {query.isError && <Callout tone="danger">{errorMessage(query.error)}</Callout>}
          {query.isSuccess && revisions.length === 0 && (
            <EmptyState
              illustration="search"
              title={t('history.empty')}
              description={t('history.emptyText')}
            />
          )}
          <ol className="flex flex-col gap-4">
            {revisions.map((revision, index) => {
              const previous = revisions[index + 1];
              const name = revision.author?.full_name ?? t('history.unknown');
              return (
                <li
                  key={revision.id}
                  className="flex flex-col gap-2.5 rounded-xl border border-border p-4"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <Avatar name={name} src={revision.author?.avatar_url} size="xs" />
                    <span className="text-sm font-medium text-fg">{name}</span>
                    <span className="text-xs text-fg-subtle">
                      {format.dateTime(revision.updated_at)}
                    </span>
                    <Badge size="sm" tone={SOURCE_TONE[revision.source]}>
                      {t(`history.source.${revision.source}`)}
                    </Badge>
                    {index === 0 && (
                      <Badge size="sm" tone="success">
                        {t('history.current')}
                      </Badge>
                    )}
                  </div>
                  <ValueDiff
                    before={previous?.field === revision.field ? previous.value : null}
                    after={revision.value}
                    field={field}
                  />
                  {onRestore && index > 0 && (
                    <Button
                      size="sm"
                      variant="secondary"
                      className="self-start"
                      leftIcon={<RotateCcw />}
                      loading={restoring === revision.id}
                      onClick={() => void onRestore(revision)}
                    >
                      {t('history.restore')}
                    </Button>
                  )}
                </li>
              );
            })}
          </ol>
          {query.hasNextPage && (
            <Button
              variant="ghost"
              className="mt-4"
              block
              loading={query.isFetchingNextPage}
              onClick={() => void query.fetchNextPage()}
            >
              {t('history.more')}
            </Button>
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
