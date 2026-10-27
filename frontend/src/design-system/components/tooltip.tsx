import { Tooltip as TooltipPrimitive } from 'radix-ui';
import type { ReactNode } from 'react';

import { cn } from '../lib/cn';

import { Kbd } from './kbd';

export const TooltipProvider = TooltipPrimitive.Provider;

interface TooltipProps {
  content: ReactNode;
  /** Keyboard shortcut shown next to the text, e.g. ['⌘', 'K']. */
  shortcut?: string[];
  side?: 'top' | 'right' | 'bottom' | 'left';
  align?: 'start' | 'center' | 'end';
  children: ReactNode;
  className?: string;
  /** Disable when the content is already visible (e.g. untruncated text). */
  disabled?: boolean;
}

export function Tooltip({
  content,
  shortcut,
  side = 'top',
  align = 'center',
  children,
  className,
  disabled = false,
}: TooltipProps) {
  if (disabled) return <>{children}</>;
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          side={side}
          align={align}
          sideOffset={6}
          collisionPadding={8}
          className={cn(
            'z-50 flex max-w-xs items-center gap-2 rounded-md bg-neutral-900 px-2.5 py-1.5 text-xs leading-snug font-medium text-white shadow-md dark:bg-neutral-100 dark:text-neutral-950',
            'origin-(--radix-tooltip-content-transform-origin) data-[state=closed]:animate-fade-out data-[state=delayed-open]:animate-scale-in',
            className,
          )}
        >
          <span>{content}</span>
          {shortcut && (
            <span className="flex gap-0.5">
              {shortcut.map((key) => (
                <Kbd
                  key={key}
                  className="border-white/15 bg-white/10 text-white dark:border-black/10 dark:bg-black/5 dark:text-neutral-700"
                >
                  {key}
                </Kbd>
              ))}
            </span>
          )}
          <TooltipPrimitive.Arrow className="fill-neutral-900 dark:fill-neutral-100" />
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
