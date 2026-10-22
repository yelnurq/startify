import { Progress as ProgressPrimitive } from 'radix-ui';
import type { ReactNode } from 'react';

import { cn } from '../lib/cn';

const fills = {
  primary: 'bg-primary',
  brand: 'bg-linear-to-r from-primary-700 to-primary-500',
  success: 'bg-linear-to-r from-success-700 to-success-500',
  gold: 'bg-gold',
  danger: 'bg-danger',
} as const;

interface ProgressProps {
  /** 0–100 */
  value: number;
  tone?: keyof typeof fills;
  size?: 'sm' | 'md';
  label?: string;
  className?: string;
}

export function Progress({ value, tone = 'brand', size = 'md', label, className }: ProgressProps) {
  const clamped = Math.max(0, Math.min(100, value));
  return (
    <ProgressPrimitive.Root
      value={clamped}
      aria-label={label}
      className={cn(
        'relative w-full overflow-hidden rounded-full bg-surface-sunken',
        size === 'sm' ? 'h-1.5' : 'h-2',
        className,
      )}
    >
      <ProgressPrimitive.Indicator
        className={cn('h-full rounded-full transition-[width] duration-500 ease-out', fills[tone])}
        style={{ width: `${clamped}%` }}
      />
    </ProgressPrimitive.Root>
  );
}

interface ProgressRingProps {
  /** 0–100 */
  value: number;
  size?: number;
  stroke?: number;
  label: string;
  children?: ReactNode;
  className?: string;
}

/** Circular progress with content in the middle (e.g. "62%"). */
export function ProgressRing({
  value,
  size = 56,
  stroke = 5,
  label,
  children,
  className,
}: ProgressRingProps) {
  const clamped = Math.max(0, Math.min(100, value));
  const radius = (size - stroke) / 2;
  const circumference = 2 * Math.PI * radius;
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(clamped)}
      className={cn('relative inline-grid shrink-0 place-items-center', className)}
      style={{ width: size, height: size }}
    >
      <svg
        width={size}
        height={size}
        viewBox={`0 0 ${size} ${size}`}
        className="-rotate-90"
        aria-hidden
      >
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={stroke}
          className="stroke-surface-sunken"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={circumference * (1 - clamped / 100)}
          className="stroke-brand transition-[stroke-dashoffset] duration-700 ease-out"
        />
      </svg>
      <span className="absolute text-xs font-semibold text-fg tabular-nums">{children}</span>
    </div>
  );
}
