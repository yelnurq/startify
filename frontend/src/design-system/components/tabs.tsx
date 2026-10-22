import { RadioGroup as RadioGroupPrimitive, Tabs as TabsPrimitive } from 'radix-ui';
import {
  createContext,
  use,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentProps,
  type CSSProperties,
  type ReactNode,
} from 'react';

import { cn } from '../lib/cn';

const TabsValue = createContext('');

/**
 * Position of the active item inside a container, kept in CSS variables so a
 * single highlight can glide between items (no animation library needed).
 */
function useActiveRect(selector: string, value: string) {
  const container = useRef<HTMLDivElement>(null);
  const [rect, setRect] = useState<{ x: number; width: number } | null>(null);
  useLayoutEffect(() => {
    const el = container.current;
    if (!el) return;
    const measure = () => {
      const active = el.querySelector<HTMLElement>(selector);
      setRect(active ? { x: active.offsetLeft, width: active.offsetWidth } : null);
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [selector, value]);
  return { container, rect };
}

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
  return (
    <TabsPrimitive.Root
      value={current}
      onValueChange={(next) => {
        setInner(next);
        onValueChange?.(next);
      }}
      {...props}
    >
      <TabsValue value={current}>{children}</TabsValue>
    </TabsPrimitive.Root>
  );
}

export function TabsList({
  className,
  children,
  ...props
}: ComponentProps<typeof TabsPrimitive.List>) {
  const value = use(TabsValue);
  const { container, rect } = useActiveRect('[role="tab"][data-state="active"]', value);
  return (
    <TabsPrimitive.List
      ref={container}
      className={cn(
        'relative flex [scrollbar-width:none] items-center gap-1 overflow-x-auto border-b border-border',
        className,
      )}
      {...props}
    >
      {children}
      {rect && (
        <span
          aria-hidden
          className="pointer-events-none absolute bottom-0 left-0 h-0.5 w-(--tab-w) translate-x-(--tab-x) rounded-full bg-primary transition-[translate,width] duration-300 ease-out motion-reduce:transition-none"
          style={
            {
              '--tab-x': `${String(rect.x + 8)}px`,
              '--tab-w': `${String(Math.max(rect.width - 16, 0))}px`,
            } as CSSProperties
          }
        />
      )}
    </TabsPrimitive.List>
  );
}

interface TabsTriggerProps extends ComponentProps<typeof TabsPrimitive.Trigger> {
  icon?: ReactNode;
  count?: number;
}

export function TabsTrigger({ className, icon, count, children, ...props }: TabsTriggerProps) {
  return (
    <TabsPrimitive.Trigger
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
  const { container, rect } = useActiveRect('[role="radio"][data-state="checked"]', value);
  return (
    <RadioGroupPrimitive.Root
      ref={container}
      aria-label={label}
      value={value}
      onValueChange={(next) => {
        onValueChange(next as T);
      }}
      orientation="horizontal"
      className={cn(
        'relative inline-flex items-center gap-0.5 rounded-lg bg-surface-sunken p-0.5',
        className,
      )}
    >
      {rect && (
        <span
          aria-hidden
          className="pointer-events-none absolute inset-y-0.5 left-0 w-(--seg-w) translate-x-(--seg-x) rounded-md bg-surface shadow-sm transition-[translate,width] duration-300 ease-out motion-reduce:transition-none dark:bg-surface-raised"
          style={
            {
              '--seg-x': `${String(rect.x)}px`,
              '--seg-w': `${String(rect.width)}px`,
            } as CSSProperties
          }
        />
      )}
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
          {option.icon}
          {option.label}
        </RadioGroupPrimitive.Item>
      ))}
    </RadioGroupPrimitive.Root>
  );
}
