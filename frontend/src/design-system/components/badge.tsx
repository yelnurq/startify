import { cva, type VariantProps } from 'class-variance-authority';
import { X } from 'lucide-react';
import type { ComponentProps, ReactNode } from 'react';

import { cn } from '../lib/cn';

export const toneClasses = {
  neutral: 'border-border bg-surface-raised text-fg-muted',
  primary: 'border-primary-border bg-primary-soft text-primary-soft-fg',
  success: 'border-success-border bg-success-soft text-success-soft-fg',
  warning: 'border-warning-border bg-warning-soft text-warning-soft-fg',
  danger: 'border-danger-border bg-danger-soft text-danger-soft-fg',
  info: 'border-info-border bg-info-soft text-info-soft-fg',
  gold: 'border-gold-border bg-gold-soft text-gold-soft-fg',
} as const;

export type Tone = keyof typeof toneClasses;

const dotClasses: Record<Tone, string> = {
  neutral: 'bg-fg-faint',
  primary: 'bg-brand',
  success: 'bg-success-500',
  warning: 'bg-warning-400',
  danger: 'bg-danger-500',
  info: 'bg-info-500',
  gold: 'bg-gold',
};

const badgeVariants = cva(
  'inline-flex max-w-full items-center gap-1.5 rounded-full border font-medium whitespace-nowrap [&_svg]:shrink-0',
  {
    variants: {
      tone: toneClasses,
      size: {
        sm: 'h-5 px-2 text-xs [&_svg]:size-3',
        md: 'h-6 px-2.5 text-xs [&_svg]:size-3.5',
        lg: 'h-7 px-3 text-sm [&_svg]:size-4',
      },
    },
    defaultVariants: { tone: 'neutral', size: 'md' },
  },
);

interface BadgeProps extends ComponentProps<'span'>, VariantProps<typeof badgeVariants> {
  /** Leading status dot. */
  dot?: boolean;
  icon?: ReactNode;
}

export function Badge({
  className,
  tone,
  size,
  dot = false,
  icon,
  children,
  ...props
}: BadgeProps) {
  return (
    <span className={cn(badgeVariants({ tone, size }), className)} {...props}>
      {dot && (
        <span aria-hidden className={cn('size-1.5 rounded-full', dotClasses[tone ?? 'neutral'])} />
      )}
      {icon}
      <span className="truncate">{children}</span>
    </span>
  );
}

interface TagProps {
  children: ReactNode;
  /** Filter chip: toggles on click and reports aria-pressed. */
  selected?: boolean;
  onToggle?: () => void;
  /** Removable chip: shows an × button with this accessible label. */
  removeLabel?: string;
  onRemove?: () => void;
  icon?: ReactNode;
  className?: string;
}

export function Tag({
  children,
  selected,
  onToggle,
  removeLabel,
  onRemove,
  icon,
  className,
}: TagProps) {
  const base = cn(
    'inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-sm font-medium transition-colors duration-150 [&_svg]:size-3.5',
    selected
      ? 'border-primary-border bg-primary-soft text-primary-soft-fg'
      : 'border-border bg-surface text-fg-muted hover:border-border-strong hover:text-fg',
    className,
  );
  if (onToggle) {
    return (
      <button type="button" aria-pressed={selected ?? false} onClick={onToggle} className={base}>
        {icon}
        {children}
      </button>
    );
  }
  return (
    <span className={base}>
      {icon}
      {children}
      {onRemove && removeLabel && (
        <button
          type="button"
          aria-label={removeLabel}
          onClick={onRemove}
          className="relative -mr-2 grid size-6 place-items-center rounded-full text-fg-subtle transition-colors hover:bg-surface-sunken hover:text-fg pointer-coarse:after:absolute pointer-coarse:after:-inset-1"
        >
          <X aria-hidden />
        </button>
      )}
    </span>
  );
}
