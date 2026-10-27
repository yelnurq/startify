import { ArrowDownRight, ArrowUpRight, Minus } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { AnimatedNumber } from './animated-number';
import { Card } from './card';
import { Sparkline } from './sparkline';

const iconTones = {
  primary: 'bg-primary-soft text-primary-soft-fg',
  gold: 'bg-gold-soft text-gold-soft-fg',
  success: 'bg-success-soft text-success-soft-fg',
  info: 'bg-info-soft text-info-soft-fg',
} as const;

interface StatTileProps {
  label: string;
  value: number;
  format: (value: number) => string;
  /** Relative change versus the previous period, e.g. 0.12 for +12 %. */
  delta?: number;
  deltaLabel?: string;
  formatDelta?: (value: number) => string;
  trend?: number[];
  icon?: ReactNode;
  tone?: keyof typeof iconTones;
  footnote?: ReactNode;
  className?: string;
}

/** KPI tile: animated value, change versus the previous period and a trend line. */
export function StatTile({
  label,
  value,
  format,
  delta,
  deltaLabel,
  formatDelta = (v) => `${(Math.abs(v) * 100).toFixed(0)}%`,
  trend,
  icon,
  tone = 'primary',
  footnote,
  className,
}: StatTileProps) {
  const { t } = useTranslation();
  const direction =
    delta === undefined || Math.abs(delta) < 0.005 ? 'flat' : delta > 0 ? 'up' : 'down';
  const DeltaIcon =
    direction === 'up' ? ArrowUpRight : direction === 'down' ? ArrowDownRight : Minus;
  return (
    <Card className={cn('gap-4 overflow-hidden p-5', className)}>
      <div className="flex items-center gap-3">
        {icon && (
          <span
            className={cn(
              'grid size-9 shrink-0 place-items-center rounded-lg [&_svg]:size-4.5',
              iconTones[tone],
            )}
          >
            {icon}
          </span>
        )}
        <p className="min-w-0 text-sm leading-snug font-medium text-fg-muted">{label}</p>
      </div>
      <div className="flex items-end justify-between gap-3">
        <AnimatedNumber
          value={value}
          format={format}
          className="font-display text-3xl font-bold text-ink"
        />
        {delta !== undefined && (
          <span
            className={cn(
              'mb-1 inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-xs font-semibold tabular-nums',
              direction === 'up' && 'bg-success-soft text-success-soft-fg',
              direction === 'down' && 'bg-danger-soft text-danger-soft-fg',
              direction === 'flat' && 'bg-surface-raised text-fg-muted',
            )}
          >
            <DeltaIcon className="size-3.5" aria-hidden />
            <span className="sr-only">{t(`trend.${direction}`)}</span>
            {formatDelta(delta)}
          </span>
        )}
      </div>
      {trend && <Sparkline data={trend} className="-mx-1" />}
      {(footnote ?? deltaLabel) && (
        <p className="-mt-1 text-xs text-fg-subtle">{footnote ?? deltaLabel}</p>
      )}
    </Card>
  );
}
