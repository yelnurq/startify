import { cn } from '../lib/cn';

/** Point of the axis `index` of `count` at `value` (0…max); axis 0 points up. */
export function radarPoint(
  index: number,
  count: number,
  value: number,
  max: number,
  radius: number,
  center: number,
): [number, number] {
  const angle = -Math.PI / 2 + (2 * Math.PI * index) / count;
  const r = (Math.max(0, Math.min(max, value)) / max) * radius;
  return [center + r * Math.cos(angle), center + r * Math.sin(angle)];
}

/** SVG `points` of a polygon through the values. */
export function radarPolygon(
  values: number[],
  max: number,
  radius: number,
  center: number,
): string {
  return values
    .map((v, i) => radarPoint(i, values.length, v, max, radius, center))
    .map(([x, y]) => `${x.toFixed(2)},${y.toFixed(2)}`)
    .join(' ');
}

/** Splits a label into lines of at most `width` characters, at most `lines` lines. */
export function wrapLabel(label: string, width = 15, lines = 4): string[] {
  const out: string[] = [];
  let line = '';
  for (const word of label.split(/\s+/).filter(Boolean)) {
    if (line && line.length + 1 + word.length > width) {
      out.push(line);
      line = word;
    } else {
      line = line ? `${line} ${word}` : word;
    }
  }
  if (line) out.push(line);
  if (out.length <= lines) return out;
  const kept = out.slice(0, lines);
  kept[lines - 1] = `${kept[lines - 1] ?? ''}…`;
  return kept;
}

export interface RadarAxis {
  key: string;
  label: string;
}

export interface RadarSeries {
  key: string;
  label: string;
  values: number[];
  /** `primary` — filled brand polygon; `muted` — dashed outline (e.g. the previous result). */
  tone?: 'primary' | 'muted';
}

interface RadarChartProps {
  axes: RadarAxis[];
  series: RadarSeries[];
  /** Upper bound of every axis. */
  max?: number;
  /** Accessible name of the chart; the values are also listed in a hidden table. */
  label: string;
  /** Number formatting of the hidden table. */
  format?: (value: number) => string;
  className?: string;
}

const SIZE = 360;
const CENTER = SIZE / 2;
const RADIUS = 104;
const LABEL_GAP = 14;
const LINE_HEIGHT = 14;
/** Room on both sides for the labels of the side axes. */
const PAD_X = 40;

/**
 * Radar (spider) chart of scores on several criteria. Pure SVG coloured with
 * theme tokens; screen readers get a table of the same numbers.
 */
export function RadarChart({
  axes,
  series,
  max = 10,
  label,
  format = String,
  className,
}: RadarChartProps) {
  const count = axes.length;
  if (count < 3) return null;
  const rings = [0.25, 0.5, 0.75, 1];
  return (
    <figure className={cn('flex flex-col items-center gap-3', className)}>
      <svg
        role="img"
        aria-label={label}
        viewBox={`${String(-PAD_X)} 0 ${String(SIZE + 2 * PAD_X)} ${String(SIZE)}`}
        className="h-auto w-full max-w-[28rem] overflow-visible"
      >
        {rings.map((ring) => (
          <polygon
            key={ring}
            points={radarPolygon(
              axes.map(() => max * ring),
              max,
              RADIUS,
              CENTER,
            )}
            className={cn('fill-none', ring === 1 ? 'stroke-border-strong' : 'stroke-border')}
            strokeWidth={1}
          />
        ))}
        {axes.map((axis, i) => {
          const [x, y] = radarPoint(i, count, max, max, RADIUS, CENTER);
          return (
            <line
              key={axis.key}
              x1={CENTER}
              y1={CENTER}
              x2={x}
              y2={y}
              className="stroke-border"
              strokeWidth={1}
            />
          );
        })}
        {series.map((s) =>
          s.tone === 'muted' ? (
            <polygon
              key={s.key}
              points={radarPolygon(s.values, max, RADIUS, CENTER)}
              className="fill-none stroke-fg-subtle"
              strokeWidth={1.5}
              strokeDasharray="4 4"
              strokeLinejoin="round"
            />
          ) : (
            <g key={s.key}>
              <polygon
                points={radarPolygon(s.values, max, RADIUS, CENTER)}
                className="fill-brand/20 stroke-brand"
                strokeWidth={2}
                strokeLinejoin="round"
              />
              {s.values.map((v, i) => {
                const [x, y] = radarPoint(i, count, v, max, RADIUS, CENTER);
                return (
                  <circle
                    key={axes[i]?.key ?? i}
                    cx={x}
                    cy={y}
                    r={3.5}
                    className="fill-surface stroke-brand"
                    strokeWidth={2}
                  />
                );
              })}
            </g>
          ),
        )}
        {axes.map((axis, i) => {
          const [x, y] = radarPoint(i, count, max, max, RADIUS + LABEL_GAP, CENTER);
          const lines = wrapLabel(axis.label);
          const anchor = Math.abs(x - CENTER) < 4 ? 'middle' : x > CENTER ? 'start' : 'end';
          // Labels above the centre grow upwards, below it downwards.
          const offset =
            y < CENTER - 4
              ? -(lines.length - 1) * LINE_HEIGHT
              : Math.abs(y - CENTER) <= 4
                ? (-(lines.length - 1) * LINE_HEIGHT) / 2
                : LINE_HEIGHT * 0.6;
          return (
            <text
              key={axis.key}
              x={x}
              y={y + offset}
              textAnchor={anchor}
              className="fill-fg-muted text-[12px] font-medium"
            >
              {lines.map((line, n) => (
                <tspan key={line} x={x} dy={n === 0 ? 0 : LINE_HEIGHT}>
                  {line}
                </tspan>
              ))}
            </text>
          );
        })}
      </svg>
      {series.length > 1 && (
        <figcaption className="flex flex-wrap justify-center gap-x-4 gap-y-1 text-xs text-fg-muted">
          {series.map((s) => (
            <span key={s.key} className="inline-flex items-center gap-1.5">
              <span
                aria-hidden
                className={cn(
                  'inline-block h-0.5 w-4 rounded-full',
                  s.tone === 'muted' ? 'border-t-2 border-dashed border-fg-subtle' : 'bg-brand',
                )}
              />
              {s.label}
            </span>
          ))}
        </figcaption>
      )}
      {/* A table sizes to its content and ignores the 1px of sr-only, so a wrapper hides it. */}
      <div className="sr-only">
        <table>
          <caption>{label}</caption>
          <thead>
            <tr>
              <th scope="col" />
              {series.map((s) => (
                <th key={s.key} scope="col">
                  {s.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {axes.map((axis, i) => (
              <tr key={axis.key}>
                <th scope="row">{axis.label}</th>
                {series.map((s) => {
                  const v = s.values[i];
                  return <td key={s.key}>{v === undefined ? '' : format(v)}</td>;
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </figure>
  );
}
