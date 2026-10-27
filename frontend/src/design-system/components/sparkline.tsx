import { useId } from 'react';

import { cn } from '../lib/cn';

const W = 120;
const H = 36;
const PAD = 3;

/** Smooth path through the points (Catmull-Rom converted to cubic Béziers). */
export function smoothPath(points: [number, number][]): string {
  if (points.length === 0) return '';
  const [first] = points;
  if (!first) return '';
  let d = `M${first[0].toFixed(2)},${first[1].toFixed(2)}`;
  for (let i = 0; i < points.length - 1; i++) {
    const p0 = points[i - 1] ?? points[i];
    const p1 = points[i];
    const p2 = points[i + 1];
    const p3 = points[i + 2] ?? p2;
    if (!p0 || !p1 || !p2 || !p3) continue;
    const c1x = p1[0] + (p2[0] - p0[0]) / 6;
    const c1y = p1[1] + (p2[1] - p0[1]) / 6;
    const c2x = p2[0] - (p3[0] - p1[0]) / 6;
    const c2y = p2[1] - (p3[1] - p1[1]) / 6;
    d += ` C${c1x.toFixed(2)},${c1y.toFixed(2)} ${c2x.toFixed(2)},${c2y.toFixed(2)} ${p2[0].toFixed(2)},${p2[1].toFixed(2)}`;
  }
  return d;
}

interface SparklineProps {
  data: number[];
  className?: string;
}

/** Decorative trend line; the numbers it summarises must be present as text. */
export function Sparkline({ data, className }: SparklineProps) {
  const id = useId();
  if (data.length < 2) return null;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  const points = data.map((v, i): [number, number] => [
    PAD + (i / (data.length - 1)) * (W - PAD * 2),
    H - PAD - ((v - min) / span) * (H - PAD * 2),
  ]);
  const line = smoothPath(points);
  const last = points[points.length - 1] ?? [0, 0];
  const area = `${line} L${last[0].toFixed(2)},${H} L${PAD},${H} Z`;
  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      preserveAspectRatio="none"
      className={cn('h-9 w-full overflow-visible text-brand', className)}
      aria-hidden
    >
      <defs>
        <linearGradient id={id} x1="0" x2="0" y1="0" y2="1">
          <stop offset="0" stopColor="currentColor" stopOpacity="0.22" />
          <stop offset="1" stopColor="currentColor" stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id})`} />
      <path
        d={line}
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
