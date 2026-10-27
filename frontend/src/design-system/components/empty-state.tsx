import type { ReactNode } from 'react';

import { illustrations, type IllustrationName } from '../illustrations';
import { cn } from '../lib/cn';

interface EmptyStateProps {
  illustration?: IllustrationName;
  title: string;
  description?: ReactNode;
  /** Main action that resolves the emptiness ("Создать проект"). */
  action?: ReactNode;
  secondaryAction?: ReactNode;
  size?: 'sm' | 'md';
  className?: string;
}

export function EmptyState({
  illustration = 'projects',
  title,
  description,
  action,
  secondaryAction,
  size = 'md',
  className,
}: EmptyStateProps) {
  const Illustration = illustrations[illustration];
  return (
    <div
      className={cn(
        'flex flex-col items-center text-center',
        size === 'md' ? 'gap-5 px-6 py-12' : 'gap-4 px-4 py-8',
        className,
      )}
    >
      <Illustration className={size === 'md' ? 'max-w-56' : 'max-w-40'} />
      <div className="flex max-w-sm flex-col gap-1.5">
        <h3 className={cn('font-semibold text-fg', size === 'md' ? 'text-lg' : 'text-base')}>
          {title}
        </h3>
        {description && <p className="text-sm leading-relaxed text-fg-muted">{description}</p>}
      </div>
      {(action ?? secondaryAction) && (
        <div className="flex flex-wrap items-center justify-center gap-2">
          {action}
          {secondaryAction}
        </div>
      )}
    </div>
  );
}
