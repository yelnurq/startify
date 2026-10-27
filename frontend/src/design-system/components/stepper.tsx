import { Check, Lock } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { cn } from '../lib/cn';

import { TruncatedText } from './truncated-text';

export type StepStatus = 'done' | 'current' | 'todo' | 'locked';

export interface StepItem {
  id: string;
  title: string;
  meta?: ReactNode;
  status: StepStatus;
}

function StepMarker({ status, index }: { status: StepStatus; index: number }) {
  return (
    <span
      aria-hidden
      className={cn(
        'relative z-10 grid size-7 shrink-0 place-items-center rounded-full text-xs font-semibold tabular-nums ring-4 ring-surface transition-colors',
        status === 'done' && 'bg-success text-success-fg',
        status === 'current' &&
          'bg-primary text-primary-fg shadow-[0_0_0_4px_var(--qd-primary-soft)]',
        status === 'todo' && 'border border-border-strong bg-surface text-fg-subtle',
        status === 'locked' &&
          'border border-dashed border-border-strong bg-surface-raised text-fg-faint',
      )}
    >
      {status === 'done' && <Check className="size-3.5" strokeWidth={3} />}
      {status === 'locked' && <Lock className="size-3" />}
      {(status === 'current' || status === 'todo') && index + 1}
    </span>
  );
}

interface StepperProps {
  steps: StepItem[];
  /** Accessible name of the navigation. */
  label: string;
  selectedId?: string;
  onSelect?: (id: string) => void;
  className?: string;
}

/** Vertical step navigation (incubator steps) with a connecting rail. */
export function Stepper({ steps, label, selectedId, onSelect, className }: StepperProps) {
  const { t } = useTranslation();
  return (
    <nav aria-label={label} className={className}>
      <ol className="relative flex flex-col">
        {steps.map((step, index) => {
          const selected = step.id === selectedId;
          const locked = step.status === 'locked';
          return (
            <li key={step.id} className="relative">
              {index < steps.length - 1 && (
                <span
                  aria-hidden
                  className={cn(
                    'absolute top-9 bottom-0 left-[1.375rem] w-0.5 -translate-x-1/2 rounded-full',
                    step.status === 'done' ? 'bg-success-300 dark:bg-success-800' : 'bg-border',
                  )}
                />
              )}
              <button
                type="button"
                disabled={locked}
                aria-current={selected ? 'step' : undefined}
                onClick={() => onSelect?.(step.id)}
                className={cn(
                  'group flex w-full items-start gap-3 rounded-lg px-2 py-2 text-left transition-colors',
                  selected ? 'bg-primary-soft' : 'hover:bg-surface-raised',
                  locked && 'cursor-not-allowed',
                )}
              >
                <StepMarker status={step.status} index={index} />
                <span className="flex min-w-0 flex-1 flex-col gap-0.5 pt-1">
                  <span
                    className={cn(
                      'text-sm leading-snug font-medium',
                      selected ? 'text-primary-soft-fg' : locked ? 'text-fg-subtle' : 'text-fg',
                    )}
                  >
                    {step.title}
                  </span>
                  <span className="sr-only">{t(`stepper.${step.status}`)}</span>
                  {step.meta && (
                    <span className={cn('text-xs', selected ? 'text-fg-muted' : 'text-fg-subtle')}>
                      {step.meta}
                    </span>
                  )}
                </span>
              </button>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

interface StageTrackProps {
  stages: { id: string; title: string; progress: number }[];
  label: string;
  className?: string;
}

/** Horizontal overview of the four incubator stages, each filled by progress. */
export function StageTrack({ stages, label, className }: StageTrackProps) {
  return (
    <ol
      aria-label={label}
      className={cn(
        'grid gap-x-2 gap-y-3',
        stages.length === 4 ? 'grid-cols-2 sm:grid-cols-4' : 'grid-cols-1',
        className,
      )}
      style={
        stages.length === 4
          ? undefined
          : { gridTemplateColumns: `repeat(${String(stages.length)}, minmax(0, 1fr))` }
      }
    >
      {stages.map((stage, index) => {
        const value = Math.max(0, Math.min(100, stage.progress));
        return (
          <li key={stage.id} className="flex min-w-0 flex-col gap-2">
            <span className="h-1.5 overflow-hidden rounded-full bg-surface-sunken">
              <span
                className={cn(
                  'block h-full rounded-full transition-[width] duration-700 ease-out',
                  value === 100 ? 'bg-success' : 'bg-linear-to-r from-primary-400 to-primary-600',
                )}
                style={{ width: `${value}%` }}
              />
            </span>
            <span className="flex min-w-0 items-baseline gap-1.5 text-xs">
              <span className="font-semibold text-fg-subtle tabular-nums">{index + 1}</span>
              <TruncatedText className={value > 0 ? 'text-fg' : 'text-fg-subtle'}>
                {stage.title}
              </TruncatedText>
            </span>
          </li>
        );
      })}
    </ol>
  );
}
