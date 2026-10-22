import { CircleAlert, CircleCheck, CloudOff, GitMerge, Loader2, PencilLine } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import type { SaveStatus } from './step-editor';

/** "Saving…", "Saved · 5 seconds ago", "Offline" — announced politely to screen readers. */
export function SaveIndicator({
  status,
  savedAt,
}: {
  status: SaveStatus;
  savedAt: number | undefined;
}) {
  const { t } = useTranslation('incubator');
  const format = useFormatters();
  // Re-render now and then so that "5 seconds ago" moves on.
  const [, setTick] = useState(0);
  useEffect(() => {
    if (status !== 'saved') return;
    const timer = setInterval(() => {
      setTick((n) => n + 1);
    }, 15_000);
    return () => {
      clearInterval(timer);
    };
  }, [status]);

  const view: Record<SaveStatus, { icon: ReactNode; text: string; tone: string }> = {
    idle: { icon: null, text: '', tone: '' },
    dirty: {
      icon: <PencilLine aria-hidden />,
      text: t('save.dirty'),
      tone: 'text-fg-subtle',
    },
    saving: {
      icon: <Loader2 className="animate-spin" aria-hidden />,
      text: t('save.saving'),
      tone: 'text-fg-subtle',
    },
    saved: {
      icon: <CircleCheck className="text-success" aria-hidden />,
      text: savedAt
        ? t('save.savedAgo', { time: format.relative(new Date(savedAt)) })
        : t('save.saved'),
      tone: 'text-fg-subtle',
    },
    offline: {
      icon: <CloudOff aria-hidden />,
      text: t('save.offline'),
      tone: 'text-warning-soft-fg',
    },
    conflict: {
      icon: <GitMerge aria-hidden />,
      text: t('save.conflict'),
      tone: 'text-warning-soft-fg',
    },
    error: {
      icon: <CircleAlert aria-hidden />,
      text: t('save.error'),
      tone: 'text-danger',
    },
  };
  const { icon, text, tone } = view[status];
  return (
    <span
      role="status"
      aria-live="polite"
      className={cn('flex items-center gap-1.5 text-xs whitespace-nowrap [&_svg]:size-3.5', tone)}
    >
      {icon}
      <span className="hidden sm:inline">{text}</span>
      <span className="sr-only sm:hidden">{text}</span>
    </span>
  );
}
