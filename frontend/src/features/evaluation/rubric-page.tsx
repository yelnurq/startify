import { useQuery, useQueryClient } from '@tanstack/react-query';
import { RotateCcw, Save } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  cn,
  Input,
  Progress,
  Skeleton,
  toast,
} from '@/design-system';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import { rubricQuery, useUpdateRubric, type Rubric } from './api';
import { DEFAULT_WEIGHTS, weightSum, weightsProblem } from './logic';

function weightsOf(rubric: Rubric): Record<string, number> {
  return Object.fromEntries(rubric.criteria.map((c) => [c.key, c.weight]));
}

/** University administrators set the weights of the evaluation rubric. */
export function RubricPage() {
  const { t } = useTranslation('evaluation');
  const rubric = useQuery(rubricQuery);
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="font-display text-3xl font-bold text-ink">{t('rubric.title')}</h1>
        <p className="text-fg-muted">{t('rubric.lead')}</p>
      </header>
      {rubric.isPending ? (
        <Skeleton className="h-96 rounded-xl" />
      ) : rubric.isError ? (
        <Callout tone="danger">{errorMessage(rubric.error)}</Callout>
      ) : (
        <RubricEditor
          key={rubric.data.etag ?? 'none'}
          rubric={rubric.data.data}
          etag={rubric.data.etag}
        />
      )}
    </div>
  );
}

function RubricEditor({ rubric, etag }: { rubric: Rubric; etag: string | undefined }) {
  const { t } = useTranslation('evaluation');
  const text = useLocalized();
  const queryClient = useQueryClient();
  const update = useUpdateRubric();
  const saved = weightsOf(rubric);
  const [weights, setWeights] = useState(saved);
  const keys = rubric.criteria.map((c) => c.key);
  const sum = weightSum(weights);
  const problem = weightsProblem(weights, keys);
  const dirty = keys.some((k) => weights[k] !== saved[k]);
  const isDefault = keys.every((k) => weights[k] === DEFAULT_WEIGHTS[k]);
  // Saving the default weights back keeps a stored rubric, which still reads as the default.
  const customized = rubric.customized && keys.some((k) => saved[k] !== DEFAULT_WEIGHTS[k]);

  const save = () => {
    update.mutate(
      { weights, etag },
      {
        onSuccess: () => {
          toast.success(t('rubric.saved'));
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 412) {
            toast.error(t('rubric.stale'));
            void queryClient.invalidateQueries({ queryKey: rubricQuery.queryKey });
            return;
          }
          toast.error(errorMessage(err));
        },
      },
    );
  };

  return (
    <Card className="gap-5 p-5 sm:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Badge tone={customized ? 'gold' : 'neutral'}>
          {customized ? t('rubric.customized') : t('rubric.default')}
        </Badge>
        <span
          aria-live="polite"
          className={cn(
            'text-sm font-semibold tabular-nums',
            sum === 100 ? 'text-success-soft-fg' : 'text-warning-soft-fg',
          )}
        >
          {t('rubric.sum', { value: sum })}
        </span>
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {rubric.criteria.map((c) => {
          const title = text(c.title);
          const value = weights[c.key] ?? 0;
          return (
            <li
              key={c.key}
              className="grid grid-cols-1 items-center gap-2 py-3 sm:grid-cols-[minmax(0,1fr)_7rem]"
            >
              <div className="flex min-w-0 flex-col gap-2">
                <label htmlFor={`weight-${c.key}`} className="text-sm font-medium text-fg">
                  {title}
                </label>
                <Progress
                  value={value}
                  size="sm"
                  label={t('rubric.weightLabel', { criterion: title })}
                />
              </div>
              <Input
                id={`weight-${c.key}`}
                type="number"
                inputMode="numeric"
                min={0}
                max={100}
                step={1}
                value={Number.isFinite(value) ? value : ''}
                aria-invalid={!Number.isInteger(value) || value < 0 || value > 100}
                onChange={(e) => {
                  const next = e.target.value === '' ? Number.NaN : Number(e.target.value);
                  setWeights((w) => ({ ...w, [c.key]: next }));
                }}
              />
            </li>
          );
        })}
      </ul>
      {problem === 'sum' && <Callout tone="warning">{t('rubric.sumBad', { value: sum })}</Callout>}
      <p className="text-xs text-fg-subtle">{t('rubric.note')}</p>
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          variant="secondary"
          leftIcon={<RotateCcw />}
          disabled={isDefault}
          onClick={() => {
            setWeights({ ...DEFAULT_WEIGHTS });
          }}
        >
          {t('rubric.reset')}
        </Button>
        <Button
          leftIcon={<Save />}
          loading={update.isPending}
          disabled={!dirty || problem !== null}
          onClick={save}
        >
          {t('rubric.save')}
        </Button>
      </div>
    </Card>
  );
}
