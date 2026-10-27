import { CircleAlert, CircleCheck, Info, TriangleAlert } from 'lucide-react';
import { Toaster as Sonner } from 'sonner';

import { useTheme } from '../theme';

export { toast } from 'sonner';

/** App-wide toasts styled with the design tokens. Use toast.success(..., { action }) for Undo. */
export function Toaster() {
  const theme = useTheme((s) => s.resolved);
  return (
    <Sonner
      theme={theme}
      position="bottom-right"
      gap={10}
      visibleToasts={4}
      icons={{
        success: <CircleCheck className="size-4.5 text-success" aria-hidden />,
        error: <CircleAlert className="size-4.5 text-danger" aria-hidden />,
        warning: <TriangleAlert className="size-4.5 text-warning-500" aria-hidden />,
        info: <Info className="size-4.5 text-info" aria-hidden />,
      }}
      toastOptions={{
        unstyled: true,
        classNames: {
          toast:
            'group flex w-full items-start gap-3 rounded-xl border border-border bg-surface p-4 text-sm text-fg shadow-lg sm:w-96',
          icon: 'mt-0.5 shrink-0',
          content: 'flex min-w-0 flex-1 flex-col gap-0.5',
          title: 'font-semibold leading-snug',
          description: 'text-fg-muted leading-snug',
          actionButton:
            'shrink-0 self-center rounded-md bg-surface-raised px-2.5 py-1 text-xs font-semibold text-fg transition-colors hover:bg-surface-sunken',
          cancelButton:
            'shrink-0 self-center rounded-md px-2.5 py-1 text-xs font-medium text-fg-muted hover:text-fg',
          closeButton: 'border-border bg-surface text-fg-subtle hover:text-fg',
        },
      }}
    />
  );
}
