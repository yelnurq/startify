import { CircleAlert, CircleCheck, Info, Sparkles, TriangleAlert } from 'lucide-react';
import type { ReactNode } from 'react';

import { cn } from '../lib/cn';

const tones = {
  info: { classes: 'border-info-border bg-info-soft text-info-soft-fg', Icon: Info },
  success: {
    classes: 'border-success-border bg-success-soft text-success-soft-fg',
    Icon: CircleCheck,
  },
  warning: {
    classes: 'border-warning-border bg-warning-soft text-warning-soft-fg',
    Icon: TriangleAlert,
  },
  danger: { classes: 'border-danger-border bg-danger-soft text-danger-soft-fg', Icon: CircleAlert },
  ai: { classes: 'border-primary-border bg-primary-soft text-primary-soft-fg', Icon: Sparkles },
} as const;

interface CalloutProps {
  tone?: keyof typeof tones;
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  icon?: ReactNode;
  className?: string;
}

/** Inline message inside the page flow (not a toast). */
export function Callout({ tone = 'info', title, children, action, icon, className }: CalloutProps) {
  const { classes, Icon } = tones[tone];
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'note'}
      className={cn('flex gap-3 rounded-xl border p-4 text-sm', classes, className)}
    >
      <span className="mt-0.5 shrink-0 [&_svg]:size-4.5">{icon ?? <Icon aria-hidden />}</span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        {title && <p className="font-semibold">{title}</p>}
        {children && <div className="leading-relaxed">{children}</div>}
        {action && <div className="mt-2 flex flex-wrap gap-2">{action}</div>}
      </div>
    </div>
  );
}
