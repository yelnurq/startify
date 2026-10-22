import type { ComponentProps, ReactNode } from 'react';

import { cn } from '../lib/cn';

import { useFieldControl } from './field';

/** Shared look of text-like controls (inputs, textareas, select triggers). */
export const controlClasses = cn(
  'w-full min-w-0 rounded-control border border-border-strong bg-surface text-sm text-fg',
  'transition-[border-color,box-shadow] duration-150 placeholder:text-fg-subtle',
  'hover:border-fg-faint',
  'focus-visible:border-primary focus-visible:ring-3 focus-visible:ring-primary-500/15 focus-visible:outline-none',
  'aria-invalid:border-danger aria-invalid:focus-visible:ring-danger/15',
  'disabled:cursor-not-allowed disabled:bg-surface-raised disabled:opacity-70',
);

interface InputProps extends ComponentProps<'input'> {
  /** Icon or text inside the field before the value. */
  leading?: ReactNode;
  /** Icon, unit or button inside the field after the value. */
  trailing?: ReactNode;
  inputSize?: 'sm' | 'md' | 'lg';
}

const heights = { sm: 'h-8', md: 'h-10', lg: 'h-12 text-base' } as const;

export function Input({
  className,
  leading,
  trailing,
  inputSize = 'md',
  id,
  required,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
  ...props
}: InputProps) {
  const a11y = useFieldControl({
    id,
    required,
    'aria-describedby': describedBy,
    'aria-invalid': invalid,
  });
  const input = (
    <input
      data-slot="input"
      className={cn(
        controlClasses,
        heights[inputSize],
        'px-3',
        leading !== undefined && 'pl-9',
        trailing !== undefined && 'pr-10',
        'file:mr-3 file:border-0 file:bg-transparent file:text-sm file:font-medium',
        className,
      )}
      {...a11y}
      {...props}
    />
  );
  if (leading === undefined && trailing === undefined) return input;
  return (
    <div className="relative flex items-center">
      {leading !== undefined && (
        <span className="pointer-events-none absolute left-3 flex text-fg-faint [&_svg]:size-4">
          {leading}
        </span>
      )}
      {input}
      {trailing !== undefined && (
        <span className="absolute right-2 flex items-center text-fg-subtle [&_svg]:size-4">
          {trailing}
        </span>
      )}
    </div>
  );
}
