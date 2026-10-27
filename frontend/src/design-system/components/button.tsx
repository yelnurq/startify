import { cva, type VariantProps } from 'class-variance-authority';
import { Slot } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { cn } from '../lib/cn';

import { Spinner } from './spinner';

/** A 32 px control reaches 44 px for a finger, only upwards and downwards (neighbours stay apart). */
const TOUCH_TALLER =
  'pointer-coarse:after:absolute pointer-coarse:after:inset-x-0 pointer-coarse:after:-inset-y-1.5';

/**
 * The university's button: square, capitals with wide tracking, a solid or
 * outlined frame. Filled accents turn navy on hover and navy turns teal.
 */
export const buttonVariants = cva(
  [
    'relative inline-flex shrink-0 items-center justify-center gap-2 rounded-control font-extrabold tracking-caps whitespace-nowrap uppercase select-none',
    'transition-[background-color,border-color,color,box-shadow,transform,filter] duration-150',
    'active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-busy:cursor-progress aria-busy:opacity-85',
    '[&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
  ],
  {
    variants: {
      variant: {
        primary:
          'border border-primary bg-primary text-primary-fg hover:border-primary-hover hover:bg-primary-hover',
        ink: 'border border-ink bg-ink text-ink-fg hover:border-ink-hover hover:bg-ink-hover',
        secondary: 'border border-border-strong bg-surface text-ink hover:border-ink',
        soft: 'bg-primary-soft text-primary-soft-fg hover:bg-primary-soft-hover',
        outline:
          'border border-primary-text text-primary-text hover:bg-primary hover:text-primary-fg',
        ghost: 'text-fg-muted hover:bg-surface-raised hover:text-ink',
        danger:
          'border border-danger bg-danger text-danger-fg hover:border-danger-hover hover:bg-danger-hover',
        gold: 'bg-gold text-gold-fg hover:brightness-[0.96]',
        /** Light outline for dark panels and photos. */
        light: 'border border-white/40 text-white hover:bg-white/10',
        link: 'h-auto! border-b border-current px-0! pb-0.5 text-primary-text hover:text-ink',
      },
      // Small sizes keep their look; on touch screens an invisible margin
      // makes them 44 px tall to the finger (WCAG 2.5.5).
      size: {
        sm: `h-8 px-3.5 text-2xs ${TOUCH_TALLER}`,
        md: 'h-10 px-5 text-xs',
        lg: 'h-12 px-6 text-xs [&_svg]:size-4.5',
        icon: 'size-10',
        'icon-sm': `size-8 ${TOUCH_TALLER}`,
      },
      block: {
        true: 'w-full',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
);

export interface ButtonProps extends ComponentProps<'button'>, VariantProps<typeof buttonVariants> {
  /** Render the single child (a link) with button styles. */
  asChild?: boolean;
  /** Shows a spinner, disables the button and announces aria-busy. */
  loading?: boolean;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
}

export function Button({
  className,
  variant,
  size,
  block,
  asChild = false,
  loading = false,
  leftIcon,
  rightIcon,
  disabled,
  type,
  children,
  ...props
}: ButtonProps) {
  const classes = cn(buttonVariants({ variant, size, block }), className);
  if (asChild) {
    return (
      <Slot.Root data-slot="button" className={classes} {...props}>
        {children}
      </Slot.Root>
    );
  }
  return (
    <button
      data-slot="button"
      type={type ?? 'button'}
      className={classes}
      disabled={disabled === true || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? <Spinner /> : leftIcon}
      {children}
      {rightIcon}
    </button>
  );
}
