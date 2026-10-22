import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import {
  ArrowDownRight,
  ArrowUpRight,
  Check,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  Download,
  FlaskConical,
  Info,
  Lightbulb,
  ListChecks,
  RefreshCw,
  ShieldAlert,
  SlidersHorizontal,
  TriangleAlert,
  UserPen,
} from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  Checkbox,
  cn,
  EmptyState,
  Progress,
  RadarChart,
  ScoreGauge,
  Skeleton,
  SkeletonText,
  Sparkline,
  Spinner,
  toast,
  TruncatedText,
} from '@/design-system';
import type { Project } from '@/features/projects/api';
import { apiFetch, ApiError, throwProblem } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { isLocale } from '@/shared/i18n';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import {
  evaluationQuery,
  evaluationsQuery,
  isRunning,
  type CriterionScore,
  type Evaluation,
  type RedFlag,
  useRequestEvaluation,
} from './api';
import {
  bySeverity,
  delta,
  experimentPlan,
  levelTone,
  severityTone,
  useExperimentChecks,
  canRequestEvaluation,
} from './logic';
import { OverrideDialog } from './override-dialog';

function useRequest(projectId: string) {
  const navigate = useNavigate();
  const request = useRequestEvaluation(projectId);
  const run = () => {
    request.mutate(undefined, {
      onSuccess: (evaluation) => {
        void navigate({
          to: '/app/projects/$projectId/evaluations/$evaluationId',
          params: { projectId, evaluationId: evaluation.id },
        });
      },
      onError: (err) => {
        const running = err instanceof ApiError ? err.problem.params?.evaluation_id : undefined;
        if (
          err instanceof ApiError &&
          err.code === 'evaluation.in_progress' &&
          typeof running === 'string'
        ) {
          toast.info(errorMessage(err));
          void navigate({
            to: '/app/projects/$projectId/evaluations/$evaluationId',
            params: { projectId, evaluationId: running },
          });
          return;
        }
        toast.error(errorMessage(err));
      },
    });
  };
  return { run, pending: request.isPending };
}

interface SectionProps {
  project: Project;
  /** A specific evaluation of the history; the latest one when absent. */
  evaluationId: string | undefined;
  /** The member manages projects (innovation manager, administrator). */
  manages: boolean;
}

/** The AI evaluation of a project: the latest one, or one from the history. */
export function EvaluationSection({ project, evaluationId, manages }: SectionProps) {
  const list = useQuery(evaluationsQuery(project.id));
  const id = evaluationId ?? list.data?.[0]?.id;
  if (!evaluationId && list.isPending) return <ReportSkeleton />;
  if (!evaluationId && list.isError) {
    return <Callout tone="danger">{errorMessage(list.error)}</Callout>;
  }
  if (!id) return <NoEvaluation project={project} manages={manages} />;
  return <EvaluationView key={id} project={project} evaluationId={id} manages={manages} />;
}

function ReportSkeleton() {
  return (
    <div className="flex flex-col gap-6" aria-busy>
      <Skeleton className="h-56 rounded-xl" />
      <SkeletonText lines={3} />
      <Skeleton className="h-72 rounded-xl" />
    </div>
  );
}

function NoEvaluation({ project, manages }: { project: Project; manages: boolean }) {
  const { t } = useTranslation('evaluation');
  const allowed = canRequestEvaluation(project, manages);
  const request = useRequest(project.id);
  const items = ['score', 'criteria', 'flags', 'experiments'] as const;
  return (
    <div className="flex flex-col gap-6">
      <EmptyState
        illustration="chart"
        title={t('empty.title')}
        description={t('empty.text')}
        action={
          allowed ? (
            <Button leftIcon={<Lightbulb />} loading={request.pending} onClick={request.run}>
              {t('cta.request')}
            </Button>
          ) : undefined
        }
      />
      {!allowed && <Callout tone="info">{t('empty.readonly')}</Callout>}
      <Card className="gap-3 p-5">
        <h2 className="text-sm font-semibold text-ink">{t('empty.includes')}</h2>
        <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {items.map((item) => (
            <li key={item} className="flex items-start gap-2 text-sm text-fg-muted">
              <Check className="mt-0.5 size-4 shrink-0 text-success" aria-hidden />
              {t(`empty.items.${item}`)}
            </li>
          ))}
        </ul>
        <p className="text-xs text-fg-subtle">{t('empty.note')}</p>
      </Card>
    </div>
  );
}

function EvaluationView({
  project,
  evaluationId,
  manages,
}: {
  project: Project;
  evaluationId: string;
  manages: boolean;
}) {
  const { t } = useTranslation('evaluation');
  const query = useQuery(evaluationQuery(project.id, evaluationId));
  const request = useRequest(project.id);
  const allowed = canRequestEvaluation(project, manages);
  if (query.isPending) return <ReportSkeleton />;
  if (query.isError) return <Callout tone="danger">{errorMessage(query.error)}</Callout>;
  const evaluation = query.data;
  if (isRunning(evaluation.status)) return <RunningEvaluation evaluation={evaluation} />;
  if (evaluation.status === 'failed') {
    return (
      <Callout
        tone="danger"
        title={t('failed.title')}
        action={
          allowed ? (
            <Button
              size="sm"
              leftIcon={<RefreshCw />}
              loading={request.pending}
              onClick={request.run}
            >
              {t('failed.retry')}
            </Button>
          ) : undefined
        }
      >
        {failureMessage(evaluation.error)}
      </Callout>
    );
  }
  return <Report project={project} evaluation={evaluation} canRequest={allowed} />;
}

/** Message of the failure code stored with an evaluation. */
function failureMessage(code: string | null): string {
  return errorMessage(
    new ApiError({
      type: 'about:blank',
      title: '',
      status: 500,
      code: code ?? 'evaluation.failed',
    }),
  );
}

function RunningEvaluation({ evaluation }: { evaluation: Evaluation }) {
  const { t } = useTranslation('evaluation');
  const stages = ['queued', 'reading', 'checking'] as const;
  const active = evaluation.status === 'pending' ? 0 : 1;
  return (
    <Card className="items-center gap-6 px-6 py-10 text-center" aria-live="polite">
      <span className="relative grid size-16 place-items-center rounded-full bg-primary-soft text-brand">
        <Spinner className="size-8" />
      </span>
      <div className="flex max-w-md flex-col gap-2">
        <h2 className="font-display text-xl font-bold text-ink">{t('running.title')}</h2>
        <p className="text-sm text-fg-muted">{t('running.text')}</p>
      </div>
      <ol className="flex w-full max-w-sm flex-col gap-3 text-left">
        {stages.map((stage, i) => {
          const state = i < active ? 'done' : i === active ? 'current' : 'todo';
          return (
            <li key={stage} className="flex items-center gap-3 text-sm">
              {state === 'done' ? (
                <CircleCheck className="size-5 shrink-0 text-success" aria-hidden />
              ) : state === 'current' ? (
                <Spinner className="size-5 shrink-0 text-brand" />
              ) : (
                <CircleDashed className="size-5 shrink-0 text-fg-subtle" aria-hidden />
              )}
              <span className={cn(state === 'todo' ? 'text-fg-subtle' : 'font-medium text-fg')}>
                {t(`running.stages.${stage}`)}
              </span>
            </li>
          );
        })}
      </ol>
    </Card>
  );
}

function reportPath(projectId: string, evaluationId: string, locale: string): string {
  return `/projects/${projectId}/evaluations/${evaluationId}/report?locale=${locale}`;
}

function fileName(response: Response): string {
  const header = response.headers.get('Content-Disposition') ?? '';
  const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(header);
  return match?.[1] ? decodeURIComponent(match[1]) : 'evaluation.pdf';
}

/** Downloads the PDF report, reporting a missing converter as a message. */
function useReportDownload(projectId: string, evaluationId: string) {
  const { i18n } = useTranslation();
  const [busy, setBusy] = useState(false);
  const download = async () => {
    setBusy(true);
    try {
      const locale = isLocale(i18n.language) ? i18n.language : 'ru';
      const response = await apiFetch(reportPath(projectId, evaluationId, locale));
      if (!response.ok) await throwProblem(response);
      const url = URL.createObjectURL(await response.blob());
      const link = document.createElement('a');
      link.href = url;
      link.download = fileName(response);
      link.click();
      setTimeout(() => {
        URL.revokeObjectURL(url);
      }, 1_000);
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };
  return { download, busy };
}

function DeltaBadge({ current, previous }: { current: number | null; previous: number | null }) {
  const { t } = useTranslation('evaluation');
  const d = delta(current, previous);
  if (!d) return <Badge tone="neutral">{t('report.first')}</Badge>;
  if (d.direction === 'same') return <Badge tone="neutral">{t('report.deltaSame')}</Badge>;
  return d.direction === 'up' ? (
    <Badge tone="success" icon={<ArrowUpRight aria-hidden />}>
      {t('report.deltaUp', { value: d.value })}
    </Badge>
  ) : (
    <Badge tone="danger" icon={<ArrowDownRight aria-hidden />}>
      {t('report.deltaDown', { value: Math.abs(d.value) })}
    </Badge>
  );
}

function Report({
  project,
  evaluation,
  canRequest,
}: {
  project: Project;
  evaluation: Evaluation;
  canRequest: boolean;
}) {
  const { t } = useTranslation('evaluation');
  const format = useFormatters();
  const request = useRequest(project.id);
  const report = useReportDownload(project.id, evaluation.id);
  const [overrideOpen, setOverrideOpen] = useState(false);
  const total = evaluation.total ?? 0;
  const level = evaluation.level ? t(`levels.${evaluation.level}`) : '';
  const corrected =
    evaluation.ai_total !== null &&
    evaluation.total !== null &&
    evaluation.ai_total !== evaluation.total;

  return (
    <div className="flex flex-col gap-8">
      <Card className="overflow-hidden">
        <div className="grid grid-cols-1 items-center gap-6 p-6 sm:grid-cols-[auto_1fr]">
          <div className="flex flex-col items-center gap-2">
            <ScoreGauge
              value={total}
              size={200}
              label={t('report.gaugeLabel', { value: total, level })}
            />
          </div>
          <div className="flex min-w-0 flex-col items-start gap-3">
            <h1 className="font-display text-2xl font-bold text-ink">{t('title')}</h1>
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone={levelTone(evaluation.level)} size="lg">
                {level}
              </Badge>
              <DeltaBadge current={evaluation.total} previous={evaluation.previous_total} />
              {corrected && (
                <Badge tone="gold" icon={<UserPen aria-hidden />}>
                  {t('report.aiTotal', { value: evaluation.ai_total })}
                </Badge>
              )}
            </div>
            <p className="text-sm text-fg-muted">
              {t('report.requested', {
                date: format.dateTime(evaluation.completed_at ?? evaluation.created_at, {
                  dateStyle: 'long',
                  timeStyle: 'short',
                }),
                name: evaluation.requested_by?.full_name ?? '—',
              })}
            </p>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                leftIcon={<Download />}
                loading={report.busy}
                onClick={() => void report.download()}
              >
                {t('report.download')}
              </Button>
              {canRequest && (
                <Button
                  variant="soft"
                  leftIcon={<RefreshCw />}
                  loading={request.pending}
                  onClick={request.run}
                >
                  {t('report.again')}
                </Button>
              )}
            </div>
          </div>
        </div>
      </Card>

      <Callout tone="ai" title={t('report.honestyTitle')}>
        <p>{t('report.honestyText')}</p>
        {evaluation.model && evaluation.prompt_version && (
          <p className="mt-1 font-mono text-xs">
            {t('report.model', { model: evaluation.model, prompt: evaluation.prompt_version })}
          </p>
        )}
      </Callout>

      {evaluation.summary && (
        <section aria-labelledby="eval-summary" className="flex flex-col gap-2">
          <h2 id="eval-summary" className="font-display text-lg font-bold text-ink">
            {t('report.summary')}
          </h2>
          <p className="leading-relaxed text-fg">{evaluation.summary}</p>
        </section>
      )}

      <Criteria
        evaluation={evaluation}
        onOverride={
          evaluation.can_override
            ? () => {
                setOverrideOpen(true);
              }
            : undefined
        }
      />
      <RedFlags flags={evaluation.red_flags} />
      <Hypotheses items={evaluation.hypotheses} />
      <Experiments evaluation={evaluation} />
      <Recommendations items={evaluation.recommendations} />
      <Overrides evaluation={evaluation} />

      {evaluation.can_override && (
        <OverrideDialog
          open={overrideOpen}
          onOpenChange={setOverrideOpen}
          projectId={project.id}
          evaluation={evaluation}
        />
      )}
    </div>
  );
}

function SectionTitle({
  id,
  icon,
  children,
  action,
}: {
  id: string;
  icon: ReactNode;
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2
        id={id}
        className="flex items-center gap-2 font-display text-lg font-bold text-ink [&_svg]:size-5 [&_svg]:text-brand"
      >
        {icon}
        {children}
      </h2>
      {action}
    </div>
  );
}

function Criteria({
  evaluation,
  onOverride,
}: {
  evaluation: Evaluation;
  onOverride: (() => void) | undefined;
}) {
  const { t } = useTranslation('evaluation');
  const text = useLocalized();
  const criteria = evaluation.criteria;
  const hasPrevious = criteria.some((c) => c.previous !== null);
  return (
    <section aria-labelledby="eval-criteria" className="flex flex-col gap-4">
      <SectionTitle
        id="eval-criteria"
        icon={<ListChecks />}
        action={
          onOverride ? (
            <Button
              size="sm"
              variant="secondary"
              leftIcon={<SlidersHorizontal />}
              onClick={onOverride}
            >
              {t('report.override')}
            </Button>
          ) : undefined
        }
      >
        {t('report.criteria')}
      </SectionTitle>
      <Card className="p-4 sm:p-6">
        <RadarChart
          label={t('report.radarLabel')}
          axes={criteria.map((c) => ({ key: c.key, label: text(c.title) }))}
          series={[
            { key: 'current', label: t('report.current'), values: criteria.map((c) => c.score) },
            ...(hasPrevious
              ? [
                  {
                    key: 'previous',
                    label: t('report.previous'),
                    values: criteria.map((c) => c.previous ?? 0),
                    tone: 'muted' as const,
                  },
                ]
              : []),
          ]}
        />
      </Card>
      <ul className="flex flex-col gap-3">
        {criteria.map((c) => (
          <CriterionRow key={c.key} criterion={c} />
        ))}
      </ul>
    </section>
  );
}

function CriterionRow({ criterion }: { criterion: CriterionScore }) {
  const { t } = useTranslation('evaluation');
  const text = useLocalized();
  const title = text(criterion.title);
  const d = delta(criterion.score, criterion.previous);
  return (
    <li className="flex flex-col gap-2 rounded-xl border border-border bg-surface p-4">
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-1">
        <div className="flex min-w-0 flex-col gap-0.5">
          <h3 className="font-semibold text-ink">{title}</h3>
          <span className="text-xs text-fg-subtle">
            {t('report.weight', { value: criterion.weight })}
          </span>
        </div>
        <div className="flex items-center gap-2">
          {d && d.direction !== 'same' && (
            <Badge tone={d.direction === 'up' ? 'success' : 'danger'} size="sm">
              {t('report.criterionDelta', {
                value: d.value > 0 ? `+${String(d.value)}` : `−${String(Math.abs(d.value))}`,
              })}
            </Badge>
          )}
          <span className="flex items-baseline gap-1">
            <span className="font-figures text-2xl font-bold text-ink tabular-nums">
              {criterion.score}
            </span>
            <span className="text-xs text-fg-subtle">{t('report.outOf')}</span>
          </span>
        </div>
      </div>
      <Progress
        value={criterion.score * 10}
        size="sm"
        label={t('report.scoreLabel', { criterion: title, value: criterion.score })}
      />
      {criterion.overridden && (
        <Badge tone="gold" size="sm" icon={<UserPen aria-hidden />} className="self-start">
          {t('report.expert', { value: criterion.ai_score })}
        </Badge>
      )}
      <p className="text-sm leading-relaxed text-fg-muted">{criterion.justification}</p>
    </li>
  );
}

const SEVERITY_ICON = { high: ShieldAlert, medium: TriangleAlert, low: Info } as const;

function RedFlags({ flags }: { flags: RedFlag[] }) {
  const { t } = useTranslation('evaluation');
  return (
    <section aria-labelledby="eval-flags" className="flex flex-col gap-4">
      <SectionTitle id="eval-flags" icon={<CircleAlert />}>
        {t('report.redFlags')}
      </SectionTitle>
      {flags.length === 0 ? (
        <Callout tone="success">{t('report.noRedFlags')}</Callout>
      ) : (
        <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {bySeverity(flags).map((flag, i) => {
            const tone = severityTone(flag.severity);
            const Icon = SEVERITY_ICON[flag.severity];
            return (
              <li
                key={`${flag.title}-${String(i)}`}
                className={cn(
                  'flex flex-col gap-2 rounded-xl border p-4',
                  tone === 'danger' && 'border-danger-border bg-danger-soft',
                  tone === 'warning' && 'border-warning-border bg-warning-soft',
                  tone === 'info' && 'border-info-border bg-info-soft',
                )}
              >
                <div className="flex items-start gap-2">
                  <Icon
                    aria-hidden
                    className={cn(
                      'mt-0.5 size-4.5 shrink-0',
                      tone === 'danger' && 'text-danger-soft-fg',
                      tone === 'warning' && 'text-warning-soft-fg',
                      tone === 'info' && 'text-info-soft-fg',
                    )}
                  />
                  <h3 className="font-semibold text-ink">{flag.title}</h3>
                </div>
                <p className="text-sm leading-relaxed text-fg-muted">{flag.detail}</p>
                <Badge tone={tone} size="sm" className="self-start">
                  {t('severity.label', { level: t(`severity.${flag.severity}`) })}
                </Badge>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function Hypotheses({ items }: { items: string[] }) {
  const { t } = useTranslation('evaluation');
  if (items.length === 0) return null;
  return (
    <section aria-labelledby="eval-hypotheses" className="flex flex-col gap-4">
      <SectionTitle id="eval-hypotheses" icon={<Lightbulb />}>
        {t('report.hypotheses')}
      </SectionTitle>
      <ol className="flex flex-col gap-2">
        {items.map((item, i) => (
          <li
            key={`${String(i)}-${item}`}
            className="flex gap-3 rounded-lg border border-border p-3 text-sm text-fg"
          >
            <span className="grid size-6 shrink-0 place-items-center rounded-full bg-surface-sunken text-xs font-semibold text-fg-muted">
              {i + 1}
            </span>
            <span className="leading-relaxed">{item}</span>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Experiments({ evaluation }: { evaluation: Evaluation }) {
  const { t } = useTranslation('evaluation');
  const checks = useExperimentChecks(evaluation.id);
  const plan = experimentPlan(evaluation.experiments);
  if (plan.length === 0) return null;
  const total = evaluation.experiments.length;
  const done = evaluation.experiments.filter((_, i) => checks.done.has(i)).length;
  return (
    <section aria-labelledby="eval-experiments" className="flex flex-col gap-4">
      <SectionTitle
        id="eval-experiments"
        icon={<FlaskConical />}
        action={
          <span className="text-sm text-fg-muted" aria-live="polite">
            {t('report.doneCount', { done, total })}
          </span>
        }
      >
        {t('report.experiments')}
      </SectionTitle>
      <ol className="relative flex flex-col gap-4 border-l-2 border-border pl-5">
        {plan.map(({ day, items }) => (
          <li key={day} className="relative flex flex-col gap-3">
            <span
              aria-hidden
              className="absolute top-1 -left-[1.6rem] size-3 rounded-full border-2 border-surface bg-brand"
            />
            <span className="text-xs font-semibold tracking-wide text-primary-text uppercase">
              {t('report.day', { day })}
            </span>
            {items.map(({ index, experiment }) => {
              const checked = checks.done.has(index);
              const id = `experiment-${evaluation.id}-${String(index)}`;
              return (
                <div
                  key={index}
                  className={cn(
                    '@container flex flex-col gap-3 rounded-xl border border-border bg-surface p-4 transition-colors',
                    checked && 'border-success-border bg-success-soft',
                  )}
                >
                  <div className="flex items-start gap-3">
                    <Checkbox
                      id={id}
                      checked={checked}
                      onCheckedChange={() => {
                        checks.toggle(index);
                      }}
                      className="mt-0.5"
                    />
                    <label
                      htmlFor={id}
                      className={cn(
                        'font-semibold text-fg',
                        checked && 'line-through decoration-fg-subtle',
                      )}
                    >
                      {experiment.title}
                      <span className="sr-only">{` — ${t('report.done')}`}</span>
                    </label>
                  </div>
                  <dl className="grid grid-cols-1 gap-3 pl-7 text-sm break-words @xl:grid-cols-3">
                    <div className="flex flex-col gap-0.5">
                      <dt className="text-xs font-medium text-fg-subtle">{t('report.how')}</dt>
                      <dd className="text-fg-muted">{experiment.how}</dd>
                    </div>
                    <div className="flex flex-col gap-0.5">
                      <dt className="text-xs font-medium text-fg-subtle">{t('report.metric')}</dt>
                      <dd className="text-fg-muted">{experiment.metric}</dd>
                    </div>
                    <div className="flex flex-col gap-0.5">
                      <dt className="text-xs font-medium text-fg-subtle">{t('report.success')}</dt>
                      <dd className="text-fg-muted">{experiment.success}</dd>
                    </div>
                  </dl>
                </div>
              );
            })}
          </li>
        ))}
      </ol>
      <p className="text-xs text-fg-subtle">{t('report.experimentsNote')}</p>
    </section>
  );
}

function Recommendations({ items }: { items: string[] }) {
  const { t } = useTranslation('evaluation');
  if (items.length === 0) return null;
  return (
    <section aria-labelledby="eval-recommendations" className="flex flex-col gap-4">
      <SectionTitle id="eval-recommendations" icon={<CircleCheck />}>
        {t('report.recommendations')}
      </SectionTitle>
      <ol className="flex flex-col gap-2">
        {items.map((item, i) => (
          <li key={`${String(i)}-${item}`} className="flex gap-3 text-sm text-fg">
            <span className="grid size-6 shrink-0 place-items-center rounded-full bg-primary text-xs font-semibold text-primary-fg">
              {i + 1}
            </span>
            <span className="pt-0.5 leading-relaxed">{item}</span>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Overrides({ evaluation }: { evaluation: Evaluation }) {
  const { t } = useTranslation('evaluation');
  const text = useLocalized();
  const format = useFormatters();
  if (evaluation.overrides.length === 0) return null;
  const titles = new Map(evaluation.criteria.map((c) => [c.key, text(c.title)]));
  return (
    <section aria-labelledby="eval-overrides" className="flex flex-col gap-4">
      <SectionTitle id="eval-overrides" icon={<UserPen />}>
        {t('report.overrides')}
      </SectionTitle>
      <ul className="flex flex-col gap-2">
        {evaluation.overrides.map((o) => (
          <li
            key={`${o.criterion}-${o.created_at}`}
            className="flex flex-col gap-1 rounded-xl border border-gold-border bg-gold-soft p-4 text-sm"
          >
            <span className="font-semibold text-gold-soft-fg">
              {t('report.overrideRow', {
                criterion: titles.get(o.criterion) ?? o.criterion,
                from: o.from,
                to: o.to,
              })}
            </span>
            <span className="text-fg">{o.comment}</span>
            <span className="text-xs text-fg-muted">
              {o.author?.full_name ?? '—'} ·{' '}
              {format.dateTime(o.created_at, { dateStyle: 'medium', timeStyle: 'short' })}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** History of a project's evaluations with the trend of the totals (side column). */
export function EvaluationHistory({
  projectId,
  activeId,
}: {
  projectId: string;
  activeId: string | undefined;
}) {
  const { t } = useTranslation('evaluation');
  const format = useFormatters();
  const list = useQuery(evaluationsQuery(projectId));
  if (list.isPending) {
    return (
      <div className="flex flex-col gap-3 p-3" aria-busy>
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </div>
    );
  }
  if (list.isError)
    return (
      <Callout tone="danger" className="m-3">
        {errorMessage(list.error)}
      </Callout>
    );
  const items = list.data;
  if (items.length === 0) {
    return <p className="p-4 text-sm text-fg-muted">{t('history.empty')}</p>;
  }
  const current = activeId ?? items[0]?.id;
  const totals = items
    .filter((e) => e.status === 'ready' && e.total !== null)
    .map((e) => e.total ?? 0)
    .reverse();
  return (
    <div className="flex h-full flex-col gap-3 overflow-y-auto p-3">
      {totals.length > 1 && (
        <div className="rounded-xl border border-border p-3">
          <Sparkline data={totals} />
          <p className="sr-only">{t('history.chart', { values: totals.join(', ') })}</p>
        </div>
      )}
      <ul className="flex flex-col gap-2">
        {items.map((e) => {
          const active = e.id === current;
          return (
            <li key={e.id}>
              <Link
                to="/app/projects/$projectId/evaluations/$evaluationId"
                params={{ projectId, evaluationId: e.id }}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'flex items-center gap-3 rounded-xl border border-border p-3 transition-colors hover:bg-surface-raised',
                  active && 'border-primary-border bg-primary-soft hover:bg-primary-soft',
                )}
              >
                <span className="grid size-11 shrink-0 place-items-center rounded-full bg-surface-sunken font-figures text-base font-bold text-ink tabular-nums">
                  {e.status === 'ready' ? (
                    e.total
                  ) : isRunning(e.status) ? (
                    <Spinner className="text-brand" />
                  ) : (
                    '—'
                  )}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <TruncatedText className="text-sm font-medium text-fg">
                    {e.status === 'ready' && e.level
                      ? t(`levels.${e.level}`)
                      : t(`status.${e.status}`)}
                  </TruncatedText>
                  <span className="text-xs text-fg-muted">
                    {format.dateTime(e.completed_at ?? e.created_at, { dateStyle: 'medium' })}
                    {e.overridden && ` · ${t('history.overridden')}`}
                  </span>
                </span>
                {active && <span className="sr-only">{t('history.current')}</span>}
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** Entry to the evaluation from the project overview. */
export function EvaluationTeaser({ project, manages }: { project: Project; manages: boolean }) {
  const { t } = useTranslation('evaluation');
  const latest = project.evaluation;
  return (
    <Card className="gap-3 p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 font-display text-lg font-bold text-ink [&_svg]:size-5 [&_svg]:text-brand">
          <ListChecks aria-hidden />
          {t('title')}
        </h2>
        {latest && (
          <Badge tone={levelTone(latest.level)}>
            {t('cta.latest', { score: latest.total, level: t(`levels.${latest.level}`) })}
          </Badge>
        )}
      </div>
      <p className="text-sm text-fg-muted">{t('cta.text')}</p>
      <div className="flex flex-wrap gap-2">
        {latest ? (
          <Button asChild variant="secondary">
            <Link to="/app/projects/$projectId/evaluation" params={{ projectId: project.id }}>
              {t('cta.open')}
            </Link>
          </Button>
        ) : (
          canRequestEvaluation(project, manages) && (
            <Button asChild>
              <Link to="/app/projects/$projectId/evaluation" params={{ projectId: project.id }}>
                {t('cta.request')}
              </Link>
            </Button>
          )
        )}
      </div>
    </Card>
  );
}
