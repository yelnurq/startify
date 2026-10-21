import { useId } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

/** Qadam ("step"): three ascending steps towards a gold goal. */
export function LogoMark({ className }: { className?: string }) {
  const id = useId();
  return (
    <svg viewBox="0 0 32 32" className={cn('size-8 shrink-0', className)} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="var(--color-primary-400)" />
          <stop offset="1" stopColor="var(--color-primary-700)" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="9" fill={`url(#${id})`} />
      <path
        d="M7.5 23.5h5.2v-5h5.2v-5h5.6"
        fill="none"
        stroke="#fff"
        strokeWidth="2.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="23.4" cy="8.6" r="2.7" fill="var(--color-gold-400)" />
    </svg>
  );
}

interface LogoProps {
  className?: string;
  /** Short tenant name shown after the wordmark, e.g. "КазУТБ". */
  tenant?: string;
}

export function Logo({ className, tenant }: LogoProps) {
  const { t } = useTranslation();
  return (
    <span className={cn('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-display text-xl font-semibold text-fg">{t('app.name')}</span>
      {tenant && (
        <span className="rounded-md border border-border bg-surface-raised px-1.5 py-0.5 text-xs font-semibold text-fg-muted">
          {tenant}
        </span>
      )}
    </span>
  );
}
