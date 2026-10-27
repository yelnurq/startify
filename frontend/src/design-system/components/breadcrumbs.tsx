import { ChevronRight } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { TruncatedText } from './truncated-text';

export interface Crumb {
  label: string;
  /** Rendered link for every crumb but the last (current page). */
  render?: (children: ReactNode) => ReactNode;
}

export function Breadcrumbs({ items, className }: { items: Crumb[]; className?: string }) {
  const { t } = useTranslation();
  return (
    <nav aria-label={t('breadcrumbs')} className={cn('min-w-0', className)}>
      <ol className="flex min-w-0 items-center gap-1.5 text-sm">
        {items.map((item, index) => {
          const last = index === items.length - 1;
          const label = <TruncatedText className="max-w-[16rem]">{item.label}</TruncatedText>;
          return (
            <li
              key={`${item.label}-${String(index)}`}
              className="flex min-w-0 items-center gap-1.5"
            >
              {last ? (
                <span aria-current="page" className="min-w-0 font-medium text-fg">
                  {label}
                </span>
              ) : (
                <span className="min-w-0 text-fg-subtle transition-colors hover:text-fg [&_a]:block">
                  {item.render ? item.render(label) : label}
                </span>
              )}
              {!last && <ChevronRight aria-hidden className="size-3.5 shrink-0 text-fg-faint" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
