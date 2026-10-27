import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

interface TableProps extends ComponentProps<'table'> {
  /** Accessible name of the scroll region (tables scroll sideways on phones). */
  label: string;
}

export function Table({ className, label, ...props }: TableProps) {
  return (
    <div
      role="region"
      aria-label={label}
      tabIndex={0}
      className="relative w-full overflow-x-auto rounded-card border border-border bg-surface focus-visible:ring-4 focus-visible:ring-primary-500/20 focus-visible:outline-none"
    >
      <table
        className={cn('w-full caption-bottom border-collapse text-sm', className)}
        {...props}
      />
    </div>
  );
}

export function TableHeader({ className, ...props }: ComponentProps<'thead'>) {
  return <thead className={cn('bg-surface [&_tr]:border-b', className)} {...props} />;
}

export function TableBody({ className, ...props }: ComponentProps<'tbody'>) {
  return <tbody className={cn('[&_tr:last-child]:border-0', className)} {...props} />;
}

export function TableRow({ className, ...props }: ComponentProps<'tr'>) {
  return (
    <tr
      className={cn(
        'border-b border-border transition-colors hover:bg-surface-raised/50 data-[state=selected]:bg-primary-soft',
        className,
      )}
      {...props}
    />
  );
}

export function TableHead({
  className,
  numeric,
  ...props
}: ComponentProps<'th'> & { numeric?: boolean }) {
  return (
    <th
      scope="col"
      className={cn(
        'h-11 px-4 text-left align-middle text-2xs font-bold tracking-label whitespace-nowrap text-ink uppercase',
        numeric && 'text-right',
        className,
      )}
      {...props}
    />
  );
}

export function TableCell({
  className,
  numeric,
  ...props
}: ComponentProps<'td'> & { numeric?: boolean }) {
  return (
    <td
      className={cn(
        'px-4 py-3 align-middle text-fg',
        numeric && 'text-right tabular-nums',
        className,
      )}
      {...props}
    />
  );
}

export function TableCaption({ className, ...props }: ComponentProps<'caption'>) {
  return (
    <caption className={cn('px-4 py-3 text-left text-sm text-fg-subtle', className)} {...props} />
  );
}
