import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import {
  ArrowLeft,
  ArrowRight,
  BadgeCheck,
  CircleCheck,
  History,
  ListChecks,
  MessageSquare,
  MessageSquarePlus,
  ShieldCheck,
  Sparkles,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Badge,
  Button,
  Callout,
  celebrateStage,
  EmptyState,
  IconButton,
  Skeleton,
  SkeletonText,
  toast,
} from '@/design-system';
import { AiNotice } from '@/features/ai/ai-notice';
import { EvaluationTeaser } from '@/features/evaluation/evaluation-page';
import { canApprove, canEdit, type Project } from '@/features/projects/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import {
  commentsQuery,
  restoreRevision,
  stepQuery,
  useApproveStep,
  useStepAction,
  type IncubatorTemplate,
  type Revision,
  type StepGap,
  type StepList,
} from './api';
import { CommentsSheet } from './comments-sheet';
import { StepField } from './fields';
import { HistorySheet } from './history-sheet';
import { useStepEditor } from './hooks';
import { MaterialsPanel } from './materials';
import { mentionablePeople } from './mentions';
import { STEP_FRESH_MS } from './prefetch';
import type { SaveStatus, Tagged } from './step-editor';
import { LeanCanvas, UnitEconomicsPanel } from './step-extras';
import { ApproveDialog, CheckCard, ConflictDialog } from './step-panels';
import { findStep, isDone, neighbours, stageOf, type FlatStep } from './template';
import type { FieldValue, TemplateField } from './values';

/** What the mentor panel needs from the open step form. */
export interface StepBridge {
  apply: (field: string, value: FieldValue) => Promise<void>;
}

interface StepPageProps {
  project: Project;
  template: IncubatorTemplate;
  stepKey: string;
  steps: StepList | undefined;
  meId: string;
  innovationManager: boolean;
  /** Newest version of the step seen through presence. */
  latestVersion: number | undefined;
  onStatus: (status: SaveStatus, savedAt: number | undefined) => void;
  onBridge: (bridge: StepBridge | null) => void;
}

/** A step of the incubator: loads the latest answers, then edits them. */
export function StepPage(props: StepPageProps) {
  const { t } = useTranslation('incubator');
  const step = findStep(props.template, props.stepKey);
  const query = useQuery({
    ...stepQuery(props.project.id, props.stepKey),
    // Editing starts from the server's latest version: the route loader has
    // just fetched it, anything older is fetched again first.
    staleTime: STEP_FRESH_MS,
    refetchOnWindowFocus: false,
    enabled: step !== undefined,
  });
  // Answers an earlier visit's editor left in the cache carry no server time.
  const current =
    query.data !== undefined && (query.dataUpdatedAt > 0 || query.isFetchedAfterMount);
  if (!step) {
    return (
      <EmptyState
        illustration="search"
        title={t('step.notFound')}
        action={
          <Button asChild variant="secondary">
            <Link to="/app/projects/$projectId" params={{ projectId: props.project.id }}>
              {t('nav.overview')}
            </Link>
          </Button>
        }
      />
    );
  }
  if (query.isError) return <Callout tone="danger">{errorMessage(query.error)}</Callout>;
  if (!query.data || !current) {
    return (
      <div className="flex flex-col gap-6" aria-busy>
        <Skeleton className="h-24 rounded-xl" />
        <SkeletonText lines={3} />
        <Skeleton className="h-40 rounded-xl" />
        <Skeleton className="h-40 rounded-xl" />
      </div>
    );
  }
  return <StepEditorView {...props} step={step} initial={query.data} />;
}

function StepEditorView({
  project,
  template,
  stepKey,
  steps,
  meId,
  innovationManager,
  latestVersion,
  onStatus,
  onBridge,
  step,
  initial,
}: StepPageProps & { step: FlatStep; initial: Tagged }) {
  const { t } = useTranslation(['incubator', 'common']);
  const text = useLocalized();
  const projectId = project.id;
  const { editor, snapshot } = useStepEditor(projectId, stepKey, initial, meId);
  const editable = canEdit(project);
  const approver = canApprove(project, innovationManager);
  const check = useStepAction(projectId, stepKey, 'check');
  const complete = useStepAction(projectId, stepKey, 'complete');
  const approve = useApproveStep(projectId, stepKey);
  const comments = useQuery(commentsQuery(projectId, stepKey));
  const [showGaps, setShowGaps] = useState(false);
  const [commentsFor, setCommentsFor] = useState<string | null | undefined>(undefined);
  const [historyFor, setHistoryFor] = useState<TemplateField | undefined>(undefined);
  const [approving, setApproving] = useState(false);
  const [restoring, setRestoring] = useState<string | null>(null);
  const stage = stageOf(template, stepKey);
  const { previous, next } = neighbours(template, stepKey);
  const saved = snapshot.step;
  const people = useMemo(() => mentionablePeople(project.members, meId), [project.members, meId]);

  useEffect(() => {
    onStatus(snapshot.status, snapshot.savedAt);
  }, [snapshot.status, snapshot.savedAt, onStatus]);

  // Edits that had not reached the server on an earlier visit are back.
  const restored = snapshot.restored;
  useEffect(() => {
    if (restored) toast.info(t('save.restored'), { id: `step-restored-${stepKey}` });
  }, [restored, stepKey, t]);

  useEffect(() => {
    if (!editable) return;
    onBridge({ apply: (field, value) => editor.apply(field, value) });
    return () => {
      onBridge(null);
    };
  }, [editable, editor, onBridge]);

  // Others saved a newer version: load it quietly when there is nothing unsaved.
  const behind = latestVersion !== undefined && latestVersion > saved.version;
  useEffect(() => {
    if (behind) void editor.refresh();
  }, [behind, editor]);

  const openComments = useMemo(() => {
    const counts = new Map<string, number>();
    for (const c of comments.data ?? []) {
      if (c.parent_id || c.resolved) continue;
      const key = c.field_key ?? '';
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return counts;
  }, [comments.data]);

  const gapMessage = (gap: StepGap) => {
    if (gap.code === 'minLength') return t('step.gap.minLength', { count: gap.count ?? 0 });
    if (gap.code === 'minItems') return t('step.gap.minItems', { count: gap.count ?? 0 });
    return t('step.gap.required');
  };
  const openTotal = [...openComments.values()].reduce((a, b) => a + b, 0);
  const errors = new Map<string, string>();
  for (const e of snapshot.fieldErrors) {
    const key = e.field.split('.')[1];
    if (key && !errors.has(key)) {
      errors.set(
        key,
        t(`common:validation.${e.code}` as 'common:validation.invalid', e.params ?? {}),
      );
    }
  }
  if (showGaps) {
    for (const gap of saved.missing) {
      if (!errors.has(gap.field)) errors.set(gap.field, gapMessage(gap));
    }
  }

  const runCheck = async () => {
    await editor.flush();
    check.mutate(undefined, {
      onSuccess: (result) => {
        editor.replace(result);
      },
      onError: (err) => {
        toast.error(errorMessage(err));
      },
    });
  };

  const runComplete = async () => {
    await editor.flush();
    complete.mutate(undefined, {
      onSuccess: (result) => {
        editor.replace(result);
        setShowGaps(false);
        const stageDone =
          stage?.steps.every((s) =>
            s.key === stepKey ? true : isDone(steps?.items.find((i) => i.key === s.key)?.status),
          ) ?? false;
        if (stageDone) {
          celebrateStage();
          toast.success(t('step.stageDone', { stage: stage ? text(stage.title) : '' }));
        } else {
          toast.success(t('step.completedToast'));
        }
      },
      onError: (err) => {
        if (err instanceof ApiError && err.code === 'incubator.step_incomplete') {
          setShowGaps(true);
          const first = err.fieldErrors[0]?.field.split('.')[1] ?? saved.missing[0]?.field;
          if (first) document.getElementById(`field-${first}`)?.scrollIntoView({ block: 'center' });
        }
        toast.error(errorMessage(err));
      },
    });
  };

  const restore = async (revision: Revision) => {
    setRestoring(revision.id);
    try {
      await editor.flush();
      const result = await restoreRevision(projectId, stepKey, revision.id, editor.currentEtag);
      editor.replace(result);
      toast.success(t('history.restored'));
      setHistoryFor(undefined);
    } catch (err) {
      if (err instanceof ApiError && err.status === 412) void editor.refresh();
      toast.error(errorMessage(err));
    } finally {
      setRestoring(null);
    }
  };

  const done = isDone(saved.status);
  const checkStale = saved.check !== null && saved.check.version !== saved.version;

  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-2">
        <p className="text-sm font-medium text-primary-text">
          {t('step.of', {
            number: step.number,
            total: template.stages.reduce((n, s) => n + s.steps.length, 0),
            stage: stage ? text(stage.title) : '',
          })}
        </p>
        <h1 className="font-display text-3xl font-bold text-balance text-ink">
          {text(step.title)}
        </h1>
        <p className="text-fg-muted">{text(step.goal)}</p>
        <div className="flex flex-wrap items-center gap-2 pt-1">
          <Badge tone="gold" icon={<Sparkles />}>
            {t('step.method', { method: text(step.method) })}
          </Badge>
          {saved.status === 'approved' && (
            <Badge tone="success" icon={<ShieldCheck />}>
              {t('status.approved')}
            </Badge>
          )}
          {saved.status === 'completed' && (
            <Badge tone="success" icon={<CircleCheck />}>
              {t('status.completed')}
            </Badge>
          )}
        </div>
      </header>

      {project.archived && <Callout tone="warning">{t('workspace.archived')}</Callout>}
      {!project.archived && !editable && <Callout tone="info">{t('workspace.readOnly')}</Callout>}
      {saved.status === 'approved' && (
        <Callout
          tone="success"
          title={t('step.approvedTitle', { name: saved.approved_by?.full_name ?? '' })}
        >
          {saved.approval_comment}
        </Callout>
      )}
      {behind && snapshot.dirty.length > 0 && (
        <Callout tone="info">{t('save.updatedByOthers')}</Callout>
      )}

      <Accordion type="single" collapsible>
        <AccordionItem value="criteria" className="rounded-xl border border-border px-4">
          <AccordionTrigger className="py-3">
            <span className="flex items-center gap-2">
              <ListChecks className="size-4 text-brand" aria-hidden />
              {t('step.criteria')}
            </span>
          </AccordionTrigger>
          <AccordionContent>
            <ul className="flex list-disc flex-col gap-1.5 pb-2 pl-5 text-sm text-fg-muted">
              {step.criteria.map((c, i) => (
                <li key={i}>{text(c.text)}</li>
              ))}
            </ul>
          </AccordionContent>
        </AccordionItem>
      </Accordion>

      <form
        className="flex flex-col gap-7"
        onSubmit={(e) => {
          e.preventDefault();
        }}
      >
        {step.fields.map((field) => {
          const count = openComments.get(field.key) ?? 0;
          const label = text(field.label);
          return (
            <StepField
              key={field.key}
              id={`field-${field.key}`}
              field={field}
              value={snapshot.values[field.key] as FieldValue}
              disabled={!editable || snapshot.status === 'conflict'}
              error={errors.get(field.key)}
              onChange={(value) => {
                editor.set(field.key, value);
              }}
              actions={
                <>
                  {count > 0 ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 px-2 text-info-soft-fg"
                      leftIcon={<MessageSquare />}
                      aria-label={t('fields.commentsCount', { field: label, count })}
                      onClick={() => {
                        setCommentsFor(field.key);
                      }}
                    >
                      {count}
                    </Button>
                  ) : (
                    <IconButton
                      size="sm"
                      label={t('fields.comments', { field: label })}
                      icon={<MessageSquarePlus />}
                      onClick={() => {
                        setCommentsFor(field.key);
                      }}
                    />
                  )}
                  <IconButton
                    size="sm"
                    label={t('fields.history', { field: label })}
                    icon={<History />}
                    onClick={() => {
                      setHistoryFor(field);
                    }}
                  />
                </>
              }
            />
          );
        })}
      </form>

      {stepKey === 'unit_economics' && <UnitEconomicsPanel values={snapshot.values} />}
      {stepKey === 'business_model' && (
        <LeanCanvas projectId={projectId} template={template} values={snapshot.values} />
      )}
      {stepKey === 'pitch' && (
        <section aria-labelledby="pitch-materials" className="flex flex-col gap-3">
          <h2 id="pitch-materials" className="font-display text-lg font-bold text-ink">
            {t('materials.title')}
          </h2>
          <p className="text-sm text-fg-muted">{t('materials.text')}</p>
          <MaterialsPanel projectId={projectId} canCreate={editable} />
        </section>
      )}
      {stepKey === 'pitch' && <EvaluationTeaser project={project} manages={innovationManager} />}

      {editable && <AiNotice externalOnly />}
      {saved.check && <CheckCard check={saved.check} stale={checkStale} />}

      <div className="sticky bottom-0 z-10 -mx-4 -mb-6 flex flex-col gap-3 border-t border-border surface-glass px-4 py-3 sm:-mx-6 sm:px-6 lg:-mx-10 lg:-mb-8 lg:px-10">
        {!done && saved.missing.length > 0 && (
          <p className="text-xs text-fg-subtle">
            {t('step.left', {
              fields: saved.missing
                .map((g) => {
                  const f = step.fields.find((x) => x.key === g.field);
                  return f ? text(f.label) : g.field;
                })
                .join(', '),
            })}
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2">
          {editable && (
            <Button
              variant="secondary"
              leftIcon={<Sparkles />}
              loading={check.isPending}
              onClick={() => void runCheck()}
            >
              {t('step.check')}
            </Button>
          )}
          {editable && !done && (
            <Button
              leftIcon={<CircleCheck />}
              loading={complete.isPending}
              onClick={() => void runComplete()}
            >
              {t('step.complete')}
            </Button>
          )}
          {approver && saved.status !== 'approved' && (
            <Button
              variant="outline"
              leftIcon={<BadgeCheck />}
              onClick={() => {
                setApproving(true);
              }}
            >
              {t('step.approve')}
            </Button>
          )}
          <Button
            variant="ghost"
            leftIcon={<MessageSquare />}
            onClick={() => {
              setCommentsFor(null);
            }}
          >
            {openTotal > 0 ? t('step.comments', { count: openTotal }) : t('step.commentsNone')}
          </Button>
          <span className="ml-auto flex gap-1">
            {previous && (
              <Button asChild variant="ghost" size="sm" aria-label={t('step.previous')}>
                <Link
                  to="/app/projects/$projectId/steps/$stepKey"
                  params={{ projectId, stepKey: previous.key }}
                >
                  <ArrowLeft aria-hidden />
                  <span className="hidden md:inline">{t('step.previous')}</span>
                </Link>
              </Button>
            )}
            {next && (
              <Button asChild variant={done ? 'soft' : 'ghost'} size="sm">
                <Link
                  to="/app/projects/$projectId/steps/$stepKey"
                  params={{ projectId, stepKey: next.key }}
                >
                  {t('step.next')}
                  <ArrowRight aria-hidden />
                </Link>
              </Button>
            )}
          </span>
        </div>
      </div>

      {snapshot.conflict && (
        <ConflictDialog
          conflict={snapshot.conflict}
          step={step}
          onResolve={(choices) => void editor.resolve(choices)}
        />
      )}
      <CommentsSheet
        projectId={projectId}
        step={step}
        field={commentsFor}
        people={people}
        canComment={!project.archived}
        canModerate={project.access === 'manager' && innovationManager}
        onOpenChange={(open) => {
          if (!open) setCommentsFor(undefined);
        }}
      />
      <HistorySheet
        projectId={projectId}
        stepKey={stepKey}
        field={historyFor}
        label={historyFor ? text(historyFor.label) : ''}
        onRestore={editable ? restore : null}
        restoring={restoring}
        onOpenChange={(open) => {
          if (!open) setHistoryFor(undefined);
        }}
      />
      <ApproveDialog
        open={approving}
        onOpenChange={setApproving}
        loading={approve.isPending}
        onApprove={(comment) => {
          approve.mutate(comment, {
            onSuccess: (result) => {
              editor.replace(result);
              setApproving(false);
              toast.success(t('approve.done'));
            },
            onError: (err) => {
              toast.error(errorMessage(err));
            },
          });
        }}
      />
    </div>
  );
}
