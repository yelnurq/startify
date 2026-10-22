import { Gauge } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { Badge } from '@/design-system';

import type { EvaluationBrief } from './api';
import { levelTone } from './logic';

/** The latest evaluation of a project as a small score chip. */
export function EvaluationChip({
  evaluation,
  size = 'sm',
}: {
  evaluation: EvaluationBrief | null | undefined;
  size?: 'sm' | 'md';
}) {
  const { t } = useTranslation('evaluation');
  if (!evaluation) return null;
  const level = t(`levels.${evaluation.level}`);
  return (
    <Badge
      tone={levelTone(evaluation.level)}
      size={size}
      icon={<Gauge aria-hidden />}
      title={t('chip.label', { score: evaluation.total, level })}
    >
      <span className="sr-only">{t('chip.label', { score: evaluation.total, level })}</span>
      <span aria-hidden className="tabular-nums">
        {evaluation.total} · {level}
      </span>
    </Badge>
  );
}
