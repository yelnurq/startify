import { X } from 'lucide-react';
import { Dialog as SheetPrimitive } from 'radix-ui';
import type { ComponentProps } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { Button } from './button';

export const Sheet = SheetPrimitive.Root;
export const SheetTrigger = SheetPrimitive.Trigger;
export const SheetClose = SheetPrimitive.Close;
export const SheetTitle = SheetPrimitive.Title;
export const SheetDescription = SheetPrimitive.Description;

const sides = {
  right:
    'inset-y-0 right-0 border-l data-[state=closed]:animate-slide-out-right data-[state=open]:animate-slide-in-right',
  left: 'inset-y-0 left-0 border-r data-[state=closed]:animate-slide-out-left data-[state=open]:animate-slide-in-left',
};

const widths = {
  sm: 'w-[min(22rem,100vw)]',
  md: 'w-[min(30rem,100vw)]',
  lg: 'w-[min(44rem,100vw)]',
};

interface SheetContentProps extends ComponentProps<typeof SheetPrimitive.Content> {
  side?: keyof typeof sides;
  size?: keyof typeof widths;
  hideClose?: boolean;
}

/** Side panel for quick views, filters and mobile navigation. */
export function SheetContent({
  side = 'right',
  size = 'md',
  hideClose = false,
  className,
  children,
  ...props
}: SheetContentProps) {
  const { t } = useTranslation();
  return (
    <SheetPrimitive.Portal>
      <SheetPrimitive.Overlay className="fixed inset-0 z-50 bg-overlay data-[state=closed]:animate-fade-out data-[state=open]:animate-fade-in" />
      <SheetPrimitive.Content
        className={cn(
          'fixed z-50 flex flex-col border-border bg-surface shadow-lg outline-none',
          sides[side],
          widths[size],
          className,
        )}
        {...props}
      >
        {children}
        {!hideClose && (
          <SheetPrimitive.Close asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t('actions.close')}
              className="absolute top-4 right-4"
            >
              <X aria-hidden />
            </Button>
          </SheetPrimitive.Close>
        )}
      </SheetPrimitive.Content>
    </SheetPrimitive.Portal>
  );
}
