import { useQueries } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { Card, CardContent, CardHeader, CardTitle, cn } from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { stepQuery, type IncubatorTemplate } from './api';
import { unitEconomics } from './numbers';
import { findStep } from './template';
import { numberValue, textValue, type StepValues } from './values';

/** Margin, lifetime value and LTV/CAC computed from the unit economics answers. */
export function UnitEconomicsPanel({ values }: { values: StepValues }) {
  const { t } = useTranslation('incubator');
  const format = useFormatters();
  const result = unitEconomics({
    price: numberValue(values.price),
    variableCost: numberValue(values.variable_cost),
    cac: numberValue(values.cac),
    purchases: numberValue(values.purchases),
  });
  const money = (v: number | null) =>
    v === null ? '—' : `${format.number(v, { maximumFractionDigits: 0 })} ${t('fields.units.kzt')}`;
  const ratio = result.ratio;
  const tone =
    ratio === null
      ? 'text-fg'
      : ratio >= 3
        ? 'text-success'
        : ratio >= 1
          ? 'text-warning-soft-fg'
          : 'text-danger';
  const tiles = [
    { key: 'margin', value: money(result.margin), className: 'text-fg' },
    { key: 'ltv', value: money(result.ltv), className: 'text-fg' },
    {
      key: 'ratio',
      value: ratio === null ? '—' : format.number(ratio, { maximumFractionDigits: 1 }),
      className: tone,
    },
  ] as const;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('unit.title')}</CardTitle>
      </CardHeader>
      <CardContent className="@container flex flex-col gap-4">
        <dl className="grid grid-cols-1 gap-3 @lg:grid-cols-3">
          {tiles.map((tile) => (
            <div key={tile.key} className="rounded-lg bg-surface-raised p-3">
              <dt className="text-xs text-fg-subtle">{t(`unit.${tile.key}`)}</dt>
              <dd
                className={cn(
                  'mt-1 font-display text-xl font-semibold tabular-nums',
                  tile.className,
                )}
              >
                {tile.value}
              </dd>
            </div>
          ))}
        </dl>
        <p className="text-sm text-fg-muted">
          {ratio === null ? t('unit.hint') : ratio >= 3 ? t('unit.good') : t('unit.low')}
        </p>
      </CardContent>
    </Card>
  );
}

type BlockId =
  | 'problem'
  | 'solution'
  | 'uvp'
  | 'advantage'
  | 'segments'
  | 'metrics'
  | 'channels'
  | 'costs'
  | 'revenue';

interface CanvasBlock {
  id: BlockId;
  step: string;
  field: string;
  className: string;
}

// Classic Lean Canvas: five columns, cost and revenue across the bottom.
const BLOCKS: CanvasBlock[] = [
  {
    id: 'problem',
    step: 'problem',
    field: 'statement',
    className: '@3xl:col-span-2 @3xl:row-span-2',
  },
  { id: 'solution', step: 'solution', field: 'solution', className: '@3xl:col-span-2' },
  { id: 'uvp', step: 'solution', field: 'uvp', className: '@3xl:col-span-2 @3xl:row-span-2' },
  {
    id: 'advantage',
    step: 'solution',
    field: 'unfair_advantage',
    className: '@3xl:col-span-2',
  },
  {
    id: 'segments',
    step: 'audience',
    field: 'segments',
    className: '@3xl:col-span-2 @3xl:row-span-2',
  },
  { id: 'metrics', step: 'metrics', field: 'north_star', className: '@3xl:col-span-2' },
  { id: 'channels', step: 'business_model', field: 'channels', className: '@3xl:col-span-2' },
  { id: 'costs', step: 'business_model', field: 'costs', className: '@3xl:col-span-5' },
  { id: 'revenue', step: 'business_model', field: 'revenue', className: '@3xl:col-span-5' },
];

const OTHER_STEPS = ['problem', 'solution', 'audience', 'metrics'];

/** The business model on one canvas: earlier answers read-only, this step's live. */
export function LeanCanvas({
  projectId,
  template,
  values,
}: {
  projectId: string;
  template: IncubatorTemplate;
  values: StepValues;
}) {
  const { t } = useTranslation('incubator');
  const results = useQueries({
    queries: OTHER_STEPS.map((key) => ({ ...stepQuery(projectId, key), staleTime: 60_000 })),
  });
  const answers = (step: string): StepValues => {
    if (step === 'business_model') return values;
    const index = OTHER_STEPS.indexOf(step);
    return results[index]?.data?.data.values ?? {};
  };
  return (
    <section aria-labelledby="lean-canvas" className="@container flex flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h2 id="lean-canvas" className="font-display text-lg font-semibold text-fg">
          {t('canvas.title')}
        </h2>
        <p className="text-sm text-fg-muted">{t('canvas.text')}</p>
      </div>
      <div className="grid grid-cols-1 gap-2 @md:grid-cols-2 @3xl:grid-cols-10">
        {BLOCKS.map((block) => {
          const value = textValue(answers(block.step)[block.field]).trim();
          const source = findStep(template, block.step);
          const own = block.step === 'business_model';
          return (
            <div
              key={block.id}
              className={cn(
                'flex min-h-28 min-w-0 flex-col gap-1.5 rounded-lg border p-3',
                own ? 'border-primary-border bg-primary-soft' : 'border-border bg-surface',
                block.className,
              )}
            >
              <div className="flex items-baseline justify-between gap-2">
                <h3 className="text-xs font-semibold tracking-wide text-fg uppercase">
                  {t(`canvas.blocks.${block.id}`)}
                </h3>
                {source && !own && (
                  <Link
                    to="/app/projects/$projectId/steps/$stepKey"
                    params={{ projectId, stepKey: block.step }}
                    className="shrink-0 text-xs text-primary-text underline-offset-2 hover:underline"
                  >
                    {t('canvas.step', { number: source.number })}
                  </Link>
                )}
              </div>
              <p
                className={cn(
                  'text-sm break-words whitespace-pre-line',
                  value ? 'text-fg' : 'text-fg-subtle italic',
                )}
              >
                {value || t('canvas.empty')}
              </p>
            </div>
          );
        })}
      </div>
    </section>
  );
}
