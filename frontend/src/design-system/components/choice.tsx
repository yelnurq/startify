import { Check, Minus } from 'lucide-react';
import {
  Checkbox as CheckboxPrimitive,
  RadioGroup as RadioGroupPrimitive,
  Slider as SliderPrimitive,
  Switch as SwitchPrimitive,
} from 'radix-ui';
import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

import { useFieldControl } from './field';

const focusRing =
  'focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-primary-500/20 focus-visible:border-primary-400';

export function Checkbox({
  className,
  id,
  ...props
}: ComponentProps<typeof CheckboxPrimitive.Root>) {
  const a11y = useFieldControl({ id });
  return (
    <CheckboxPrimitive.Root
      data-slot="checkbox"
      className={cn(
        'peer grid size-4.5 shrink-0 place-items-center rounded-control border border-border-strong bg-surface transition-colors',
        'data-[state=checked]:border-primary data-[state=checked]:bg-primary data-[state=checked]:text-primary-fg',
        'data-[state=indeterminate]:border-primary data-[state=indeterminate]:bg-primary data-[state=indeterminate]:text-primary-fg',
        'disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-danger',
        focusRing,
        className,
      )}
      {...a11y}
      {...props}
    >
      <CheckboxPrimitive.Indicator className="data-[state=checked]:animate-scale-in">
        {props.checked === 'indeterminate' ? (
          <Minus className="size-3.5" strokeWidth={3} aria-hidden />
        ) : (
          <Check className="size-3.5" strokeWidth={3} aria-hidden />
        )}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

export function RadioGroup({
  className,
  ...props
}: ComponentProps<typeof RadioGroupPrimitive.Root>) {
  return <RadioGroupPrimitive.Root className={cn('grid gap-3', className)} {...props} />;
}

export function Radio({ className, ...props }: ComponentProps<typeof RadioGroupPrimitive.Item>) {
  return (
    <RadioGroupPrimitive.Item
      className={cn(
        'grid size-4.5 shrink-0 place-items-center rounded-full border border-border-strong bg-surface transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:border-primary',
        focusRing,
        className,
      )}
      {...props}
    >
      <RadioGroupPrimitive.Indicator className="size-2 rounded-full bg-primary data-[state=checked]:animate-scale-in" />
    </RadioGroupPrimitive.Item>
  );
}

export function Switch({ className, id, ...props }: ComponentProps<typeof SwitchPrimitive.Root>) {
  const a11y = useFieldControl({ id });
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent bg-neutral-300 p-0.5 transition-colors duration-200 dark:bg-neutral-700',
        'disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-primary',
        'focus-visible:ring-4 focus-visible:ring-primary-500/25 focus-visible:outline-none',
        className,
      )}
      {...a11y}
      {...props}
    >
      <SwitchPrimitive.Thumb className="block size-4 rounded-full bg-white shadow-sm transition-transform duration-250 ease-(--ease-spring) data-[state=checked]:translate-x-4" />
    </SwitchPrimitive.Root>
  );
}

interface SliderProps extends ComponentProps<typeof SliderPrimitive.Root> {
  /** Accessible names of the thumbs (one per value). */
  thumbLabels?: string[];
}

export function Slider({
  className,
  thumbLabels = [],
  value,
  defaultValue,
  ...props
}: SliderProps) {
  const count = (value ?? defaultValue ?? [0]).length;
  return (
    <SliderPrimitive.Root
      data-slot="slider"
      value={value}
      defaultValue={defaultValue}
      className={cn(
        'relative flex h-5 w-full touch-none items-center select-none data-disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <SliderPrimitive.Track className="relative h-1.5 w-full grow overflow-hidden rounded-full bg-surface-sunken">
        <SliderPrimitive.Range className="absolute h-full rounded-full bg-primary" />
      </SliderPrimitive.Track>
      {Array.from({ length: count }, (_, i) => (
        <SliderPrimitive.Thumb
          key={i}
          aria-label={thumbLabels[i]}
          className="block size-5 rounded-full border-2 border-primary bg-surface shadow-md transition-[box-shadow,transform] duration-150 hover:scale-110 focus-visible:ring-4 focus-visible:ring-primary-500/25 focus-visible:outline-none active:scale-95"
        />
      ))}
    </SliderPrimitive.Root>
  );
}
