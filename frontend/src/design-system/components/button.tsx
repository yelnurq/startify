import { cva, type VariantProps } from 'class-variance-authority';
import { Slot } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { cn } from '../lib/cn';

import { Spinner } from './spinner';

export const buttonVariants = cva(
  [
    'relative inline-flex shrink-0 items-center justify-center gap-2 font-medium whitespace-nowrap select-none',
    'transition-[background-color,border-color,color,box-shadow,transform,filter] duration-150',
    'active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-busy:cursor-progress aria-busy:opacity-85',
    '[&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
  ],
  {
    variants: {
      variant: {
        primary: 'bg-primary text-primary-fg shadow-button hover:bg-primary-hover',
        secondary:
          'border border-border bg-surface text-fg shadow-sm hover:border-border-strong hover:bg-surface-raised',
        soft: 'bg-primary-soft text-primary-soft-fg hover:bg-primary-soft-hover',
        outline: 'border border-primary-border text-primary-text hover:bg-primary-soft',
        ghost: 'text-fg-muted hover:bg-surface-raised hover:text-fg',
        danger: 'bg-danger text-danger-fg shadow-button hover:bg-danger-hover',
        gold: 'bg-gold text-gold-fg shadow-button hover:brightness-[0.96]',
        link: 'h-auto! px-0! text-primary-text underline-offset-4 hover:underline',
      },
      size: {
        sm: 'h-8 rounded-lg px-3 text-sm',
        md: 'h-10 rounded-lg px-4 text-sm',
        lg: 'h-12 rounded-xl px-6 text-base [&_svg]:size-5',
        icon: 'size-10 rounded-lg',
        'icon-sm': 'size-8 rounded-lg',
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
