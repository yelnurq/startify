import { Check, ChevronDown, ChevronUp } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';
import type { ComponentProps } from 'react';

import { cn } from '../lib/cn';

import { useFieldControl } from './field';
import { controlClasses } from './input';

export const Select = SelectPrimitive.Root;
export const SelectGroup = SelectPrimitive.Group;
export const SelectValue = SelectPrimitive.Value;

export function SelectTrigger({
  className,
  children,
  id,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
  ...props
}: ComponentProps<typeof SelectPrimitive.Trigger>) {
  const a11y = useFieldControl({ id, 'aria-describedby': describedBy, 'aria-invalid': invalid });
  const { required: _required, ...triggerA11y } = a11y;
  return (
    <SelectPrimitive.Trigger
      data-slot="select-trigger"
      className={cn(
        controlClasses,
        'flex h-10 items-center justify-between gap-2 px-3 text-left data-placeholder:text-fg-subtle [&>span]:truncate',
        className,
      )}
      {...triggerA11y}
      {...props}
    >
      {children}
      <SelectPrimitive.Icon asChild>
        <ChevronDown className="size-4 shrink-0 text-fg-faint" aria-hidden />
      </SelectPrimitive.Icon>
    </SelectPrimitive.Trigger>
  );
}

export function SelectContent({
  className,
  children,
  position = 'popper',
  ...props
}: ComponentProps<typeof SelectPrimitive.Content>) {
  return (
    <SelectPrimitive.Portal>
      <SelectPrimitive.Content
        position={position}
        sideOffset={6}
        className={cn(
          'relative z-50 max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width) overflow-hidden rounded-xl border border-border bg-surface shadow-lg',
          'origin-(--radix-select-content-transform-origin) data-[state=closed]:animate-fade-out data-[state=open]:animate-scale-in',
          className,
        )}
        {...props}
      >
        <SelectPrimitive.ScrollUpButton className="flex h-6 items-center justify-center text-fg-subtle">
          <ChevronUp className="size-4" aria-hidden />
        </SelectPrimitive.ScrollUpButton>
        <SelectPrimitive.Viewport className="p-1">{children}</SelectPrimitive.Viewport>
        <SelectPrimitive.ScrollDownButton className="flex h-6 items-center justify-center text-fg-subtle">
          <ChevronDown className="size-4" aria-hidden />
        </SelectPrimitive.ScrollDownButton>
      </SelectPrimitive.Content>
    </SelectPrimitive.Portal>
  );
}

export function SelectItem({
  className,
  children,
  ...props
}: ComponentProps<typeof SelectPrimitive.Item>) {
  return (
    <SelectPrimitive.Item
      className={cn(
        'relative flex h-9 cursor-default items-center gap-2 rounded-md pr-8 pl-2.5 text-sm text-fg outline-none select-none',
        'data-disabled:opacity-50 data-highlighted:bg-surface-raised data-[state=checked]:font-medium',
        className,
      )}
      {...props}
    >
      <SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText>
      <SelectPrimitive.ItemIndicator className="absolute right-2.5 flex text-primary-text">
        <Check className="size-4" aria-hidden />
      </SelectPrimitive.ItemIndicator>
    </SelectPrimitive.Item>
  );
}

export function SelectLabel({ className, ...props }: ComponentProps<typeof SelectPrimitive.Label>) {
  return (
    <SelectPrimitive.Label
      className={cn('px-2.5 pt-2 pb-1 text-xs font-medium text-fg-subtle', className)}
      {...props}
    />
  );
}

export function SelectSeparator({
  className,
  ...props
}: ComponentProps<typeof SelectPrimitive.Separator>) {
  return <SelectPrimitive.Separator className={cn('my-1 h-px bg-border', className)} {...props} />;
}
