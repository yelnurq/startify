import { Popover as PopoverPrimitive } from 'radix-ui';
import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

export const Popover = PopoverPrimitive.Root;
export const PopoverTrigger = PopoverPrimitive.Trigger;
export const PopoverAnchor = PopoverPrimitive.Anchor;
export const PopoverClose = PopoverPrimitive.Close;

export function PopoverContent({
  className,
  align = 'center',
  sideOffset = 8,
  ...props
}: ComponentProps<typeof PopoverPrimitive.Content>) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        align={align}
        sideOffset={sideOffset}
        collisionPadding={12}
        className={cn(
          'z-50 w-80 rounded-xl border border-border bg-surface p-4 text-sm text-fg shadow-lg outline-none',
          'origin-(--radix-popover-content-transform-origin) data-[state=closed]:animate-fade-out data-[state=open]:animate-scale-in',
          className,
        )}
        {...props}
      />
    </PopoverPrimitive.Portal>
  );
}
