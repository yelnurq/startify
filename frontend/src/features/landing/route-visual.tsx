import {
  Gauge,
  Layers,
  Lightbulb,
  PanelsTopLeft,
  Rocket,
  Trophy,
  type LucideIcon,
} from 'lucide-react';
import { useId } from 'react';
import { useTranslation } from 'react-i18next';

import { cn, smoothPath, useReducedMotion } from '@/design-system';

type NodeKey = 'idea' | 'incubator' | 'evaluation' | 'showcase' | 'demoDay' | 'market';

interface RouteNode {
  key: NodeKey;
  x: number;
  y: number;
  icon: LucideIcon;
  label: 'below' | 'above';
}

const NODES: RouteNode[] = [
  { key: 'idea', x: 58, y: 368, icon: Lightbulb, label: 'below' },
  { key: 'incubator', x: 152, y: 262, icon: Layers, label: 'above' },
  { key: 'evaluation', x: 268, y: 312, icon: Gauge, label: 'below' },
  { key: 'showcase', x: 338, y: 192, icon: PanelsTopLeft, label: 'above' },
  { key: 'demoDay', x: 448, y: 232, icon: Trophy, label: 'below' },
  { key: 'market', x: 502, y: 86, icon: Rocket, label: 'above' },
];

const PATH = smoothPath(NODES.map((n): [number, number] => [n.x, n.y]));

const CHIPS: { key: 'mentor' | 'rubric' | 'voting'; x: number; y: number }[] = [
  { key: 'mentor', x: 0, y: 150 },
  { key: 'rubric', x: 196, y: 372 },
  { key: 'voting', x: 372, y: 300 },
];

/** Hero illustration: a project travelling from an idea to the market. */
export function RouteVisual({ className }: { className?: string }) {
  const { t } = useTranslation('landing');
  const reduce = useReducedMotion();
  const gradient = useId();
  const glow = useId();
  return (
    <figure className={cn('relative', className)}>
      <svg
        viewBox="0 0 560 440"
        className="h-auto w-full overflow-visible"
        role="img"
        aria-label={t('route.label')}
      >
        <defs>
          <linearGradient id={gradient} x1="0" x2="1" y1="1" y2="0">
            <stop offset="0" stopColor="var(--color-primary-300)" />
            <stop offset="0.6" stopColor="var(--color-primary-500)" />
            <stop offset="1" stopColor="var(--color-gold-400)" />
          </linearGradient>
          <radialGradient id={glow}>
            <stop offset="0" stopColor="var(--color-gold-400)" stopOpacity="0.45" />
            <stop offset="1" stopColor="var(--color-gold-400)" stopOpacity="0" />
          </radialGradient>
        </defs>
        <circle cx={502} cy={86} r={74} fill={`url(#${glow})`} />
        <path
          d={PATH}
          fill="none"
          stroke="var(--qd-border)"
          strokeWidth={10}
          strokeLinecap="round"
        />
        <path
          d={PATH}
          fill="none"
          stroke={`url(#${gradient})`}
          strokeWidth={4}
          strokeLinecap="round"
          pathLength={1}
          strokeDasharray="1"
          className="motion-safe:animate-draw"
        />
        {!reduce &&
          [0, 2.2, 4.4].map((begin) => (
            <circle key={begin} r={5} fill="var(--color-gold-400)" opacity={0.9}>
              <animateMotion
                dur="6.6s"
                begin={`${String(begin + 1.6)}s`}
                repeatCount="indefinite"
                path={PATH}
              />
            </circle>
          ))}
        {NODES.map(({ key, x, y, icon: Icon, label }, index) => {
          const last = key === 'market';
          return (
            <g
              key={key}
              className="motion-safe:animate-rise"
              style={{ animationDelay: `${String(0.25 + index * 0.3)}s` }}
            >
              <circle
                cx={x}
                cy={y}
                r={27}
                fill={last ? 'var(--qd-gold)' : 'var(--qd-surface)'}
                stroke={last ? 'var(--color-gold-500)' : 'var(--qd-border-strong)'}
                strokeWidth={1.5}
              />
              <Icon
                x={x - 11}
                y={y - 11}
                width={22}
                height={22}
                strokeWidth={2}
                color={last ? 'var(--qd-gold-fg)' : 'var(--qd-primary-text)'}
                aria-hidden
              />
              <text
                x={x}
                y={label === 'below' ? y + 48 : y - 40}
                textAnchor="middle"
                className="fill-fg font-sans text-[13px] font-semibold"
              >
                {t(`route.${key}`)}
              </text>
            </g>
          );
        })}
        {CHIPS.map(({ key, x, y }) => (
          <foreignObject key={key} x={x} y={y} width={160} height={34}>
            <span className="inline-flex h-8 items-center gap-1.5 rounded-full border border-border bg-surface/90 px-3 text-xs font-medium whitespace-nowrap text-fg-muted shadow-sm backdrop-blur">
              <span className="size-1.5 rounded-full bg-brand" aria-hidden />
              {t(`route.chips.${key}`)}
            </span>
          </foreignObject>
        ))}
      </svg>
    </figure>
  );
}

/** Phones: the same route as a vertical timeline. */
export function RouteTimeline({ className }: { className?: string }) {
  const { t } = useTranslation('landing');
  return (
    <ol aria-label={t('route.label')} className={cn('relative flex flex-col gap-3', className)}>
      <span
        aria-hidden
        className="absolute top-5 bottom-5 left-5 w-0.5 -translate-x-1/2 rounded-full bg-linear-to-b from-primary-300 via-primary-500 to-gold-400"
      />
      {NODES.map(({ key, icon: Icon }) => (
        <li key={key} className="relative flex items-center gap-4">
          <span
            className={cn(
              'grid size-10 shrink-0 place-items-center rounded-full border shadow-sm',
              key === 'market'
                ? 'border-gold-500 bg-gold text-gold-fg'
                : 'border-border-strong bg-surface text-primary-text',
            )}
          >
            <Icon className="size-4.5" aria-hidden />
          </span>
          <span className="font-medium text-fg">{t(`route.${key}`)}</span>
        </li>
      ))}
    </ol>
  );
}
