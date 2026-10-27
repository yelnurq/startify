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
          <stop offset="0" stopColor="var(--color-primary-700)" />
          <stop offset="1" stopColor="var(--color-ink-800)" />
        </linearGradient>
      </defs>
      <rect width="32" height="32" rx="8" fill={`url(#${id})`} />
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

/** The university's seal when it has one, otherwise the Qadam mark. */
export function BrandMark({
  src,
  className,
  inverted = false,
}: {
  src?: string | null;
  className?: string;
  /** On a dark panel the seal sits on a white disc. */
  inverted?: boolean;
}) {
  if (!src) return <LogoMark className={className} />;
  return (
    <img
      src={src}
      alt=""
      width={64}
      height={64}
      className={cn(
        'size-8 shrink-0 rounded-full object-contain',
        inverted && 'bg-white p-0.5',
        className,
      )}
    />
  );
}

interface LogoProps {
  className?: string;
  /** Line of capitals under the wordmark, e.g. the university's full name. */
  caption?: string;
  /** White wordmark for dark panels. */
  inverted?: boolean;
  /** The university's seal, shown instead of the Qadam mark. */
  logoUrl?: string | null;
}

export function Logo({ className, caption, inverted = false, logoUrl }: LogoProps) {
  const { t } = useTranslation();
  const markSize = logoUrl ? 'size-10 sm:size-12' : 'sm:size-10';
  return (
    <span className={cn('inline-flex items-center gap-3', className)}>
      <BrandMark src={logoUrl} inverted={inverted} className={caption ? markSize : undefined} />
      <span className="flex min-w-0 flex-col">
        <span
          className={cn(
            'font-display text-xl leading-tight font-extrabold',
            inverted ? 'text-white' : 'text-ink',
          )}
        >
          {t('app.name')}
        </span>
        {caption && (
          <span
            className={cn(
              'hidden max-w-64 text-3xs leading-snug font-bold tracking-label uppercase sm:line-clamp-2',
              inverted ? 'text-white/70' : 'text-fg-muted',
            )}
          >
            {caption}
          </span>
        )}
      </span>
    </span>
  );
}
