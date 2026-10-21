import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

/** Placeholder block shaped like the content it replaces. */
export function Skeleton({ className, ...props }: ComponentProps<'div'>) {
  return <div aria-hidden className={cn('shimmer rounded-md', className)} {...props} />;
}

export function SkeletonText({ lines = 3, className }: { lines?: number; className?: string }) {
  return (
    <div aria-hidden className={cn('flex flex-col gap-2', className)}>
      {Array.from({ length: lines }, (_, i) => (
        <Skeleton key={i} className={cn('h-3.5', i === lines - 1 ? 'w-3/5' : 'w-full')} />
      ))}
    </div>
  );
}

/** Skeleton of a project card: cover, title, tagline, meta row. */
export function SkeletonCard({ className }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={cn(
        'flex flex-col overflow-hidden rounded-xl border border-border bg-surface',
        className,
      )}
    >
      <Skeleton className="aspect-[16/9] rounded-none" />
      <div className="flex flex-col gap-3 p-5">
        <Skeleton className="h-5 w-2/3" />
        <SkeletonText lines={2} />
        <div className="flex items-center gap-2 pt-1">
          <Skeleton className="size-6 rounded-full" />
          <Skeleton className="h-3.5 w-24" />
          <Skeleton className="ml-auto h-6 w-16 rounded-full" />
        </div>
      </div>
    </div>
  );
}
