import type { ReactNode } from 'react';

import { cn } from '../lib/cn';

interface ChatBubbleProps {
  /** Avatar or icon of the author. */
  avatar: ReactNode;
  /** The person's own messages sit on the right in the primary colour. */
  own?: boolean;
  /** Author and time above the bubble. */
  meta?: ReactNode;
  /** Muted look for stopped or failed answers. */
  muted?: boolean;
  children: ReactNode;
  className?: string;
}

/** One message of a conversation (the AI mentor, comments). */
export function ChatBubble({
  avatar,
  own = false,
  meta,
  muted = false,
  children,
  className,
}: ChatBubbleProps) {
  return (
    <div className={cn('flex gap-2.5', own && 'flex-row-reverse', className)}>
      <span className="shrink-0">{avatar}</span>
      <div className={cn('flex min-w-0 flex-col gap-1', own ? 'items-end' : 'items-start')}>
        {meta && <span className="text-xs text-fg-subtle">{meta}</span>}
        <div
          className={cn(
            'flex max-w-full flex-col gap-2 rounded-2xl px-3.5 py-2.5 text-sm leading-relaxed break-words',
            own
              ? 'rounded-tr-sm bg-primary text-primary-fg'
              : 'rounded-tl-sm bg-surface-raised text-fg',
            muted && 'opacity-75',
          )}
        >
          {children}
        </div>
      </div>
    </div>
  );
}
