import { Link, useNavigate } from '@tanstack/react-router';
import { Flame, Gauge, LayoutDashboard, MessageSquare } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  cn,
  Progress,
  ProgressRing,
  Stepper,
  type StepItem,
} from '@/design-system';
import type { EvaluationBrief } from '@/features/evaluation/api';
import { levelTone } from '@/features/evaluation/logic';
import { useLocalized } from '@/shared/i18n/localized';

import type { IncubatorTemplate, StepList, StepSummary } from './api';
import { isDone } from './template';

function stepStatus(summary: StepSummary | undefined): StepItem['status'] {
  if (isDone(summary?.status)) return 'done';
  if (summary?.status === 'in_progress') return 'current';
  return 'todo';
}

interface StepNavProps {
  projectId: string;
  template: IncubatorTemplate;
  steps: StepList | undefined;
  activeKey: string | undefined;
  /** Which page of the project is open. */
  activeView?: 'overview' | 'step' | 'evaluation';
  /** The latest evaluation, shown on its entry. */
  evaluation?: EvaluationBrief | null;
  /** Called after navigating, e.g. to close the sheet on phones. */
  onNavigate?: () => void;
}

/** Stages with their steps, progress and open comments (left column). */
export function StepNav({
  projectId,
  template,
  steps,
  activeKey,
  activeView = activeKey === undefined ? 'overview' : 'step',
  evaluation,
  onNavigate,
}: StepNavProps) {
  const { t } = useTranslation('incubator');
  const { t: te } = useTranslation('evaluation');
  const text = useLocalized();
  const navigate = useNavigate();
  const byKey = new Map(steps?.items.map((s) => [s.key, s]));
  const progress = steps?.progress;
  const activeStage =
    template.stages.find((stage) => stage.steps.some((s) => s.key === activeKey))?.key ??
    template.stages.find((stage) => stage.key === stageOfCurrent(template, progress?.current_step))
      ?.key;

  return (
    <div className="flex flex-col gap-3">
      <Link
        to="/app/projects/$projectId"
        params={{ projectId }}
        activeOptions={{ exact: true }}
        onClick={onNavigate}
        className={cn(
          'flex items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-raised',
          activeView === 'overview' && 'border-primary-border bg-primary-soft',
        )}
      >
        <ProgressRing value={progress?.percent ?? 0} size={48} label={t('nav.progress')}>
          {progress ? `${String(progress.percent)}%` : ''}
        </ProgressRing>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-1.5 text-sm font-semibold text-fg">
            <LayoutDashboard className="size-4 text-fg-subtle" aria-hidden />
            {t('nav.overview')}
          </span>
          <span className="text-xs text-fg-muted">
            {t('nav.done', {
              done: progress?.completed_steps ?? 0,
              total: progress?.total_steps ?? 14,
            })}
          </span>
          {progress && progress.streak_days > 1 && (
            <span className="flex items-center gap-1 text-xs font-medium text-gold-soft-fg">
              <Flame className="size-3.5" aria-hidden />
              {t('nav.streak', { count: progress.streak_days })}
            </span>
          )}
        </span>
      </Link>

      <Link
        to="/app/projects/$projectId/evaluation"
        params={{ projectId }}
        onClick={onNavigate}
        aria-current={activeView === 'evaluation' ? 'page' : undefined}
        className={cn(
          'flex items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-raised',
          activeView === 'evaluation' && 'border-primary-border bg-primary-soft',
        )}
      >
        <span
          className={cn(
            'grid size-12 shrink-0 place-items-center rounded-full font-display text-base font-semibold tabular-nums',
            evaluation ? 'bg-surface-sunken text-fg' : 'bg-surface-sunken text-fg-subtle',
          )}
        >
          {evaluation ? evaluation.total : <Gauge className="size-5" aria-hidden />}
        </span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-sm font-semibold text-fg">{te('title')}</span>
          {evaluation ? (
            <span
              className={cn(
                'text-xs font-medium',
                levelTone(evaluation.level) === 'success'
                  ? 'text-success-soft-fg'
                  : 'text-fg-muted',
              )}
            >
              <span className="sr-only">{te('nav.score', { score: evaluation.total })} · </span>
              {te(`levels.${evaluation.level}`)}
            </span>
          ) : (
            <span className="text-xs text-fg-muted">{te('nav.none')}</span>
          )}
        </span>
      </Link>

      <Accordion type="multiple" defaultValue={activeStage ? [activeStage] : []}>
        {template.stages.map((stage, stageIndex) => {
          const done = stage.steps.filter((s) => isDone(byKey.get(s.key)?.status)).length;
          const items: StepItem[] = stage.steps.map((s) => {
            const summary = byKey.get(s.key);
            const status = stepStatus(summary);
            const comments = summary?.open_comments ?? 0;
            const meta =
              summary?.status === 'approved'
                ? t('status.approved')
                : summary?.status === 'completed'
                  ? t('status.completed')
                  : summary && summary.filled > 0
                    ? t('nav.fields', { filled: summary.filled, total: summary.total })
                    : undefined;
            return {
              id: s.key,
              title: `${String(s.number)}. ${text(s.title)}`,
              status,
              meta:
                meta || comments ? (
                  <span className="flex flex-wrap items-center gap-x-2">
                    {meta}
                    {comments > 0 && (
                      <span className="inline-flex items-center gap-0.5 text-info-soft-fg">
                        <MessageSquare className="size-3" aria-hidden />
                        <span className="sr-only">{t('nav.comments', { count: comments })}</span>
                        <span aria-hidden>{comments}</span>
                      </span>
                    )}
                  </span>
                ) : undefined,
            };
          });
          return (
            <AccordionItem key={stage.key} value={stage.key} className="border-0">
              <AccordionTrigger className="px-2 py-3">
                <span className="flex flex-1 flex-col gap-1.5 pr-2 text-left">
                  <span className="flex items-baseline justify-between gap-2">
                    <span>
                      <span className="mr-1.5 text-fg-subtle tabular-nums">{stageIndex + 1}</span>
                      {text(stage.title)}
                    </span>
                    <span className="text-xs font-normal text-fg-subtle tabular-nums">
                      {done}/{stage.steps.length}
                    </span>
                  </span>
                  <Progress
                    value={(done / stage.steps.length) * 100}
                    size="sm"
                    tone={done === stage.steps.length ? 'success' : 'brand'}
                    label={text(stage.title)}
                  />
                </span>
              </AccordionTrigger>
              <AccordionContent className="pb-2">
                <Stepper
                  steps={items}
                  label={text(stage.title)}
                  selectedId={activeKey}
                  onSelect={(key) => {
                    void navigate({
                      to: '/app/projects/$projectId/steps/$stepKey',
                      params: { projectId, stepKey: key },
                    });
                    onNavigate?.();
                  }}
                />
              </AccordionContent>
            </AccordionItem>
          );
        })}
      </Accordion>
    </div>
  );
}

function stageOfCurrent(template: IncubatorTemplate, key: string | undefined): string | undefined {
  return template.stages.find((stage) => stage.steps.some((s) => s.key === key))?.key;
}
