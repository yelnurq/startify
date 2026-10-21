import { X } from 'lucide-react';
import { AlertDialog as AlertDialogPrimitive, Dialog as DialogPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { Button } from './button';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

const overlayClasses =
  'fixed inset-0 z-50 bg-overlay backdrop-blur-[2px] data-[state=closed]:animate-fade-out data-[state=open]:animate-fade-in';

// Centered card on desktop, bottom sheet on phones.
const panelClasses = cn(
  'fixed z-50 flex max-h-[min(90dvh,48rem)] w-full flex-col gap-0 border border-border bg-surface shadow-lg outline-none',
  'inset-x-0 bottom-0 rounded-t-2xl pb-[env(safe-area-inset-bottom)] data-[state=closed]:animate-fade-out data-[state=open]:animate-slide-up',
  'sm:inset-x-auto sm:top-1/2 sm:bottom-auto sm:left-1/2 sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-2xl sm:pb-0 sm:data-[state=open]:animate-scale-in',
);

const widths = { sm: 'sm:max-w-sm', md: 'sm:max-w-lg', lg: 'sm:max-w-2xl', xl: 'sm:max-w-4xl' };

interface DialogContentProps extends ComponentProps<typeof DialogPrimitive.Content> {
  size?: keyof typeof widths;
  /** Hide the × button (e.g. when the footer already has a close action). */
  hideClose?: boolean;
}

export function DialogContent({
  className,
  children,
  size = 'md',
  hideClose = false,
  ...props
}: DialogContentProps) {
  const { t } = useTranslation();
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className={overlayClasses} />
      <DialogPrimitive.Content className={cn(panelClasses, widths[size], className)} {...props}>
        {children}
        {!hideClose && (
          <DialogPrimitive.Close asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={t('actions.close')}
              className="absolute top-4 right-4"
            >
              <X aria-hidden />
            </Button>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export function DialogHeader({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('flex flex-col gap-1.5 px-6 pt-6 pr-14', className)} {...props} />;
}

export function DialogTitle({ className, ...props }: ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      className={cn('text-lg leading-snug font-semibold text-fg', className)}
      {...props}
    />
  );
}

export function DialogDescription({
  className,
  ...props
}: ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description className={cn('text-sm text-fg-muted', className)} {...props} />
  );
}

export function DialogBody({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('flex-1 overflow-y-auto px-6 py-5', className)} {...props} />;
}

export function DialogFooter({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      className={cn(
        'flex flex-col-reverse gap-2 border-t border-border bg-surface-raised/50 px-6 py-4 sm:flex-row sm:justify-end sm:rounded-b-2xl',
        className,
      )}
      {...props}
    />
  );
}

interface ConfirmDialogProps {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  trigger?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  tone?: 'primary' | 'danger';
  loading?: boolean;
  onConfirm: () => void;
}

/** Asks for confirmation of a consequential action; focus starts on Cancel. */
export function ConfirmDialog({
  open,
  onOpenChange,
  trigger,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = 'primary',
  loading = false,
  onConfirm,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  return (
    <AlertDialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      {trigger && <AlertDialogPrimitive.Trigger asChild>{trigger}</AlertDialogPrimitive.Trigger>}
      <AlertDialogPrimitive.Portal>
        <AlertDialogPrimitive.Overlay className={overlayClasses} />
        <AlertDialogPrimitive.Content className={cn(panelClasses, widths.sm)}>
          <div className="flex flex-col gap-2 px-6 pt-6 pb-5">
            <AlertDialogPrimitive.Title className="text-lg font-semibold text-fg">
              {title}
            </AlertDialogPrimitive.Title>
            {description && (
              <AlertDialogPrimitive.Description className="text-sm text-fg-muted">
                {description}
              </AlertDialogPrimitive.Description>
            )}
          </div>
          <div className="flex flex-col-reverse gap-2 px-6 pb-6 sm:flex-row sm:justify-end">
            <AlertDialogPrimitive.Cancel asChild>
              <Button variant="secondary">{cancelLabel ?? t('actions.cancel')}</Button>
            </AlertDialogPrimitive.Cancel>
            <Button
              variant={tone === 'danger' ? 'danger' : 'primary'}
              loading={loading}
              onClick={onConfirm}
            >
              {confirmLabel}
            </Button>
          </div>
        </AlertDialogPrimitive.Content>
      </AlertDialogPrimitive.Portal>
    </AlertDialogPrimitive.Root>
  );
}
