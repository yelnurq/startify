import { LayoutGroup, motion } from 'motion/react';
import { RadioGroup as RadioGroupPrimitive, Tabs as TabsPrimitive } from 'radix-ui';
import { createContext, use, useId, useState, type ComponentProps, type ReactNode } from 'react';

import { cn } from '../lib/cn';

const TabsValue = createContext<{ value: string; group: string }>({ value: '', group: '' });

interface TabsProps extends Omit<
  ComponentProps<typeof TabsPrimitive.Root>,
  'value' | 'defaultValue'
> {
  value?: string;
  defaultValue?: string;
}

/** Tabs with an underline that glides to the active tab. */
export function Tabs({ value, defaultValue = '', onValueChange, children, ...props }: TabsProps) {
  const [inner, setInner] = useState(defaultValue);
  const current = value ?? inner;
  const group = useId();
  return (
    <TabsPrimitive.Root
      value={current}
      onValueChange={(next) => {
        setInner(next);
        onValueChange?.(next);
      }}
      {...props}
    >
      <TabsValue value={{ value: current, group }}>
        <LayoutGroup id={group}>{children}</LayoutGroup>
      </TabsValue>
    </TabsPrimitive.Root>
  );
}

export function TabsList({ className, ...props }: ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      className={cn(
        'relative flex [scrollbar-width:none] items-center gap-1 overflow-x-auto border-b border-border',
        className,
      )}
      {...props}
    />
  );
}

interface TabsTriggerProps extends ComponentProps<typeof TabsPrimitive.Trigger> {
  icon?: ReactNode;
  count?: number;
}

export function TabsTrigger({
  className,
  value,
  icon,
  count,
  children,
  ...props
}: TabsTriggerProps) {
  const ctx = use(TabsValue);
  const active = ctx.value === value;
  return (
    <TabsPrimitive.Trigger
      value={value}
      className={cn(
        'relative flex h-10 shrink-0 items-center gap-2 rounded-t-md px-3 text-sm font-medium whitespace-nowrap text-fg-subtle transition-colors',
        'hover:text-fg data-[state=active]:text-fg [&_svg]:size-4',
        className,
      )}
      {...props}
    >
      {icon}
      {children}
      {count !== undefined && (
        <span className="rounded-full bg-surface-sunken px-1.5 text-xs text-fg-muted tabular-nums">
          {count}
        </span>
      )}
      {active && (
        <motion.span
          layoutId="tab-indicator"
          className="absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-primary"
          transition={{ type: 'spring', stiffness: 520, damping: 40 }}
        />
      )}
    </TabsPrimitive.Trigger>
  );
}

export function TabsContent({ className, ...props }: ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      className={cn('pt-5 outline-none data-[state=active]:animate-fade-in', className)}
      {...props}
    />
  );
}

interface SegmentedControlProps<T extends string> {
  value: T;
  onValueChange: (value: T) => void;
  options: { value: T; label: ReactNode; icon?: ReactNode }[];
  /** Accessible name of the group. */
  label: string;
  size?: 'sm' | 'md';
  className?: string;
}

/** Mutually exclusive options (view modes, periods) with a sliding highlight. */
export function SegmentedControl<T extends string>({
  value,
  onValueChange,
  options,
  label,
  size = 'md',
  className,
}: SegmentedControlProps<T>) {
  const group = useId();
  return (
    <RadioGroupPrimitive.Root
      aria-label={label}
      value={value}
      onValueChange={(next) => {
        onValueChange(next as T);
      }}
      orientation="horizontal"
      className={cn(
        'inline-flex items-center gap-0.5 rounded-lg bg-surface-sunken p-0.5',
        className,
      )}
    >
      <LayoutGroup id={group}>
        {options.map((option) => (
          <RadioGroupPrimitive.Item
            key={option.value}
            value={option.value}
            className={cn(
              'relative flex items-center justify-center gap-1.5 rounded-md px-3 font-medium whitespace-nowrap text-fg-muted transition-colors',
              'hover:text-fg data-[state=checked]:text-fg [&_svg]:size-4',
              size === 'sm' ? 'h-7 text-xs' : 'h-8 text-sm',
            )}
          >
            {value === option.value && (
              <motion.span
                layoutId="segment"
                className="absolute inset-0 rounded-md bg-surface shadow-sm dark:bg-surface-raised"
                transition={{ type: 'spring', stiffness: 520, damping: 40 }}
              />
            )}
            <span className="relative flex items-center gap-1.5">
              {option.icon}
              {option.label}
            </span>
          </RadioGroupPrimitive.Item>
        ))}
      </LayoutGroup>
    </RadioGroupPrimitive.Root>
  );
}
