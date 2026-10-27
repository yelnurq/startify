import { Check, ChevronRight, Circle } from 'lucide-react';
import { DropdownMenu as MenuPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';

import { cn } from '../lib/cn';

export const DropdownMenu = MenuPrimitive.Root;
export const DropdownMenuTrigger = MenuPrimitive.Trigger;
export const DropdownMenuGroup = MenuPrimitive.Group;
export const DropdownMenuSub = MenuPrimitive.Sub;
export const DropdownMenuRadioGroup = MenuPrimitive.RadioGroup;

const contentClasses = cn(
  'z-50 min-w-48 overflow-hidden rounded-xl border border-border bg-surface p-1 text-sm text-fg shadow-lg outline-none',
  'origin-(--radix-dropdown-menu-content-transform-origin) data-[state=closed]:animate-fade-out data-[state=open]:animate-scale-in',
);

const itemClasses = cn(
  'relative flex h-9 cursor-default items-center gap-2.5 rounded-md px-2.5 outline-none select-none',
  'data-disabled:pointer-events-none data-disabled:opacity-50 data-highlighted:bg-surface-raised',
  '[&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-fg-subtle',
);

export function DropdownMenuContent({
  className,
  sideOffset = 6,
  ...props
}: ComponentProps<typeof MenuPrimitive.Content>) {
  return (
    <MenuPrimitive.Portal>
      <MenuPrimitive.Content
        sideOffset={sideOffset}
        collisionPadding={8}
        className={cn(contentClasses, className)}
        {...props}
      />
    </MenuPrimitive.Portal>
  );
}

interface ItemProps extends ComponentProps<typeof MenuPrimitive.Item> {
  icon?: ReactNode;
  shortcut?: string;
  destructive?: boolean;
}

export function DropdownMenuItem({
  className,
  icon,
  shortcut,
  destructive,
  children,
  ...props
}: ItemProps) {
  return (
    <MenuPrimitive.Item
      className={cn(
        itemClasses,
        destructive &&
          'text-danger-soft-fg data-highlighted:bg-danger-soft [&_svg]:text-danger-soft-fg',
        className,
      )}
      {...props}
    >
      {icon}
      <span className="flex-1 truncate">{children}</span>
      {shortcut && <span className="text-xs tracking-wider text-fg-subtle">{shortcut}</span>}
    </MenuPrimitive.Item>
  );
}

export function DropdownMenuCheckboxItem({
  className,
  children,
  ...props
}: ComponentProps<typeof MenuPrimitive.CheckboxItem>) {
  return (
    <MenuPrimitive.CheckboxItem className={cn(itemClasses, 'pl-8', className)} {...props}>
      <span className="absolute left-2.5 flex">
        <MenuPrimitive.ItemIndicator>
          <Check className="text-primary-text!" aria-hidden />
        </MenuPrimitive.ItemIndicator>
      </span>
      {children}
    </MenuPrimitive.CheckboxItem>
  );
}

export function DropdownMenuRadioItem({
  className,
  children,
  icon,
  ...props
}: ComponentProps<typeof MenuPrimitive.RadioItem> & { icon?: ReactNode }) {
  return (
    <MenuPrimitive.RadioItem className={cn(itemClasses, className)} {...props}>
      {icon}
      <span className="flex-1">{children}</span>
      <MenuPrimitive.ItemIndicator className="flex">
        <Circle className="size-2! fill-current text-primary-text!" aria-hidden />
      </MenuPrimitive.ItemIndicator>
    </MenuPrimitive.RadioItem>
  );
}

export function DropdownMenuLabel({
  className,
  ...props
}: ComponentProps<typeof MenuPrimitive.Label>) {
  return (
    <MenuPrimitive.Label
      className={cn('px-2.5 pt-2 pb-1 text-xs font-medium text-fg-subtle', className)}
      {...props}
    />
  );
}

export function DropdownMenuSeparator({
  className,
  ...props
}: ComponentProps<typeof MenuPrimitive.Separator>) {
  return (
    <MenuPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-border', className)} {...props} />
  );
}

export function DropdownMenuSubTrigger({
  className,
  children,
  icon,
  ...props
}: ComponentProps<typeof MenuPrimitive.SubTrigger> & { icon?: ReactNode }) {
  return (
    <MenuPrimitive.SubTrigger
      className={cn(itemClasses, 'data-[state=open]:bg-surface-raised', className)}
      {...props}
    >
      {icon}
      <span className="flex-1">{children}</span>
      <ChevronRight aria-hidden />
    </MenuPrimitive.SubTrigger>
  );
}

export function DropdownMenuSubContent({
  className,
  ...props
}: ComponentProps<typeof MenuPrimitive.SubContent>) {
  return (
    <MenuPrimitive.Portal>
      <MenuPrimitive.SubContent className={cn(contentClasses, className)} {...props} />
    </MenuPrimitive.Portal>
  );
}
