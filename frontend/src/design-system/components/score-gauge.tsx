import { useEffect, useId, useState } from 'react';

import { cn } from '../lib/cn';
import { useReducedMotion } from '../lib/reduced-motion';

const SWEEP = 240; // degrees of the dial, open at the bottom
const START = 90 + (360 - SWEEP) / 2; // measured clockwise from 3 o'clock

function polar(cx: number, cy: number, r: number, deg: number): [number, number] {
  const rad = (deg * Math.PI) / 180;
  return [cx + r * Math.cos(rad), cy + r * Math.sin(rad)];
}

function arc(cx: number, cy: number, r: number, from: number, to: number): string {
  const [x1, y1] = polar(cx, cy, r, from);
  const [x2, y2] = polar(cx, cy, r, to);
  const large = to - from > 180 ? 1 : 0;
  return `M${x1.toFixed(2)} ${y1.toFixed(2)} A${r} ${r} 0 ${large} 1 ${x2.toFixed(2)} ${y2.toFixed(2)}`;
}

interface ScoreGaugeProps {
  /** 0–100 */
  value: number;
  /** Text under the number, e.g. the readiness level. */
  caption?: string;
  /** Accessible description of the whole gauge. */
  label: string;
  size?: number;
  className?: string;
}

/**
 * Evaluation score dial. The arc reveals a danger → warning → primary → success
 * gradient up to the score, so colour follows the result.
 */
export function ScoreGauge({ value, caption, label, size = 220, className }: ScoreGaugeProps) {
  const id = useId();
  const reduce = useReducedMotion();
  const clamped = Math.max(0, Math.min(100, value));
  const [shown, setShown] = useState(reduce ? clamped : 0);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setShown(clamped);
    });
    return () => {
      cancelAnimationFrame(frame);
    };
  }, [clamped]);

  const stroke = size * 0.07;
  const r = size / 2 - stroke;
  const c = size / 2;
  const path = arc(c, c, r, START, START + SWEEP);
  const length = (Math.PI * r * SWEEP) / 180;

  return (
    <div
      role="img"
      aria-label={label}
      className={cn('relative inline-grid place-items-center', className)}
    >
      <svg width={size} height={size * 0.86} viewBox={`0 0 ${size} ${size * 0.86}`} aria-hidden>
        <defs>
          <linearGradient id={id} x1="0" x2="1" y1="0" y2="0">
            <stop offset="0" stopColor="var(--color-danger-500)" />
            <stop offset="0.38" stopColor="var(--color-warning-400)" />
            <stop offset="0.68" stopColor="var(--color-primary-400)" />
            <stop offset="1" stopColor="var(--color-success-500)" />
          </linearGradient>
        </defs>
        <path
          d={path}
          fill="none"
          strokeWidth={stroke}
          strokeLinecap="round"
          className="stroke-surface-sunken"
        />
        {[0, 25, 50, 75, 100].map((tick) => {
          const deg = START + (SWEEP * tick) / 100;
          const [x1, y1] = polar(c, c, r - stroke * 0.95, deg);
          const [x2, y2] = polar(c, c, r - stroke * 1.35, deg);
          return (
            <line
              key={tick}
              x1={x1}
              y1={y1}
              x2={x2}
              y2={y2}
              strokeWidth="1.5"
              strokeLinecap="round"
              className="stroke-border-strong"
            />
          );
        })}
        <path
          d={path}
          fill="none"
          stroke={`url(#${id})`}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={length}
          strokeDashoffset={length * (1 - shown / 100)}
          style={{
            transition: reduce ? 'none' : 'stroke-dashoffset 1.2s cubic-bezier(0.16, 1, 0.3, 1)',
          }}
        />
      </svg>
      <div className="absolute inset-x-0 top-[34%] flex flex-col items-center gap-1 text-center">
        <span className="flex items-baseline gap-1">
          <span className="font-figures text-5xl font-bold text-ink tabular-nums">
            {Math.round(clamped)}
          </span>
          <span className="text-sm font-medium text-fg-subtle">/100</span>
        </span>
        {caption && <span className="text-sm font-medium text-fg-muted">{caption}</span>}
      </div>
    </div>
  );
}
