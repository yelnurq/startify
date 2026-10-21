import type { ReactNode } from 'react';

import { cn } from '@/design-system';

interface SectionProps {
  id: string;
  index: number;
  title: string;
  lead: string;
  children: ReactNode;
}

export function Section({ id, index, title, lead, children }: SectionProps) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="flex scroll-mt-24 flex-col gap-6">
      <header className="flex flex-col gap-2">
        <span className="font-display text-sm font-semibold text-primary-text tabular-nums">
          {String(index).padStart(2, '0')}
        </span>
        <h2
          id={`${id}-title`}
          className="text-2xl font-semibold tracking-tight text-fg sm:text-3xl"
        >
          {title}
        </h2>
        <p className="max-w-2xl text-base text-fg-muted">{lead}</p>
      </header>
      {children}
    </section>
  );
}

interface SpecimenProps {
  label: string;
  children: ReactNode;
  className?: string;
  /** Content area classes (layout of the demo). */
  bodyClassName?: string;
}

/** A labelled demo panel inside a section. */
export function Specimen({ label, children, className, bodyClassName }: SpecimenProps) {
  return (
    <div
      className={cn(
        'overflow-hidden rounded-xl border border-border bg-surface shadow-sm',
        className,
      )}
    >
      <p className="border-b border-border bg-surface-raised/50 px-5 py-2.5 text-xs font-medium tracking-wide text-fg-subtle uppercase">
        {label}
      </p>
      <div className={cn('flex-1 p-5 sm:p-6', bodyClassName)}>{children}</div>
    </div>
  );
}
