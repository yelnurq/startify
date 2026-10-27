import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

interface KickerProps extends ComponentProps<'p'> {
  /** Mint on dark panels. */
  tone?: 'default' | 'light';
}

/** Kicker above a section title: a short rule and spaced capitals. */
export function Kicker({ className, tone = 'default', ...props }: KickerProps) {
  return (
    <p
      data-slot="kicker"
      className={cn(
        'flex items-center gap-3 text-xs font-bold tracking-kicker uppercase',
        'before:h-px before:w-7 before:shrink-0 before:bg-current',
        tone === 'light' ? 'text-primary-300' : 'text-primary-text',
        className,
      )}
      {...props}
    />
  );
}
