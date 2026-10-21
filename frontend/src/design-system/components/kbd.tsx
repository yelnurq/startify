import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

/** Keyboard key, used in tooltips and the command palette. */
export function Kbd({ className, ...props }: ComponentProps<'kbd'>) {
  return (
    <kbd
      className={cn(
        'inline-flex h-5 min-w-5 items-center justify-center rounded-xs border border-border bg-surface-raised px-1 font-sans text-xs leading-none font-medium text-fg-muted',
        className,
      )}
      {...props}
    />
  );
}

/** Platform-aware modifier: ⌘ on Apple devices, Ctrl elsewhere. */
export function modKey(): string {
  const platform =
    (navigator as Navigator & { userAgentData?: { platform?: string } }).userAgentData?.platform ??
    navigator.userAgent;
  return /mac|iphone|ipad/i.test(platform) ? '⌘' : 'Ctrl';
}
