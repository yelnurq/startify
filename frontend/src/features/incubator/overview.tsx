import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import {
  ArrowRight,
  Award,
  CircleCheck,
  FileCheck2,
  Flame,
  MessageSquare,
  PencilLine,
  Rocket,
  ShieldCheck,
  Sparkles,
  UserPlus,
  Users,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Button,
  Callout,
  Card,
  cn,
  EmptyState,
  ProgressRing,
  SkeletonText,
  StageTrack,
  Tooltip,
  TruncatedText,
} from '@/design-system';
import {
  activityQuery,
  BADGES,
  canEdit,
  type Badge as BadgeName,
  type Project,
} from '@/features/projects/api';
import { EvaluationTeaser } from '@/features/evaluation/evaluation-page';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { groupActivity, type ActivityGroup } from './activity';
import type { IncubatorTemplate } from './api';
import { MaterialsPanel } from './materials';
import { findStep } from './template';

const BADGE_ICONS: Record<BadgeName, typeof Award> = {
  idea_stage: Sparkles,
  market_stage: Users,
  product_stage: Rocket,
  launch_stage: Award,
  first_review: CircleCheck,
  materials_ready: FileCheck2,
  team_of_three: UserPlus,
};

/** Milestones earned and still ahead, quietly. */
export function BadgeList({ earned }: { earned: BadgeName[] }) {
  const { t } = useTranslation('incubator');
  return (
    <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
      {BADGES.map((badge) => {
        const Icon = BADGE_ICONS[badge];
        const has = earned.includes(badge);
        return (
          <li key={badge}>
            <Tooltip content={t(`badges.${badge}.hint`)}>
              <span
                tabIndex={0}
                className={cn(
                  'flex items-center gap-3 rounded-lg border p-2.5 text-sm',
                  has
                    ? 'border-gold-border bg-gold-soft text-gold-soft-fg'
                    : 'border-dashed border-border text-fg-subtle',
                )}
              >
                <Icon className="size-4 shrink-0" aria-hidden />
                <span className="flex-1">{t(`badges.${badge}.title`)}</span>
                <span className="sr-only">{has ? t('badges.earned') : t('badges.locked')}</span>
              </span>
            </Tooltip>
          </li>
        );
      })}
    </ul>
  );
}

const ACTIVITY_ICONS = {
  edit: PencilLine,
  comment: MessageSquare,
  step_completed: CircleCheck,
  step_approved: ShieldCheck,
  member_joined: UserPlus,
} as const;

/** Recent changes, comments and finished steps of a project. */
export function ActivityFeed({
  projectId,
  template,
  limit = 30,
  compact = false,
}: {
  projectId: string;
  template: IncubatorTemplate | undefined;
  limit?: number;
  compact?: boolean;
}) {
  const { t } = useTranslation('incubator');
  const query = useQuery(activityQuery(projectId, limit));
  if (query.isPending) return <SkeletonText lines={5} />;
  if (query.isError) return <Callout tone="danger">{errorMessage(query.error)}</Callout>;
  if (query.data.length === 0) {
    return (
      <EmptyState
        size="sm"
        illustration="inbox"
        title={t('activity.empty')}
        description={compact ? undefined : t('activity.emptyText')}
      />
    );
  }
  return (
    <ol className="flex flex-col gap-1">
      {groupActivity(query.data).map((item, index) => (
        <ActivityRow key={index} projectId={projectId} item={item} template={template} />
      ))}
    </ol>
  );
}

function ActivityRow({
  projectId,
  item,
  template,
}: {
  projectId: string;
  item: ActivityGroup;
  template: IncubatorTemplate | undefined;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const format = useFormatters();
  const Icon = ACTIVITY_ICONS[item.kind];
  const step = item.step_key && template ? findStep(template, item.step_key) : undefined;
  const labels = item.fields.map((key) => {
    const f = step?.fields.find((x) => x.key === key);
    return f ? text(f.label) : key;
  });
  const shown =
    labels.length > 3
      ? `${labels.slice(0, 3).join(', ')} ${t('activity.more', { count: labels.length - 3 })}`
      : labels.join(', ');
  const name = item.actor?.full_name ?? t('activity.someone');
  const where = [step ? text(step.title) : '', shown].filter(Boolean).join(' · ');
  return (
    <li className="flex gap-3 rounded-lg p-2">
      <span className="relative shrink-0">
        <Avatar name={name} src={item.actor?.avatar_url} size="sm" />
        <span className="absolute -right-1 -bottom-1 grid size-4.5 place-items-center rounded-full bg-surface ring-2 ring-surface">
          <Icon className="size-3 text-fg-muted" aria-hidden />
        </span>
      </span>
      <div className="flex min-w-0 flex-col gap-0.5">
        <p className="text-sm text-fg">
          <span className="font-medium">{name}</span>{' '}
          <span className="text-fg-muted">{t(`activity.kind.${item.kind}`)}</span>
        </p>
        {where && step && (
          <Link
            to="/app/projects/$projectId/steps/$stepKey"
            params={{ projectId, stepKey: step.key }}
            className="text-xs break-words text-primary-text underline-offset-2 hover:underline"
          >
            {where}
          </Link>
        )}
        <span className="text-xs text-fg-subtle">{format.relative(item.at)}</span>
      </div>
    </li>
  );
}

/** The project at a glance: stages, the next step, milestones, activity, materials. */
export function ProjectOverview({
  project,
  template,
  manages,
}: {
  project: Project;
  template: IncubatorTemplate;
  /** The member manages projects (innovation manager, administrator). */
  manages: boolean;
}) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const progress = project.progress;
  const current = findStep(template, progress.current_step);
  const allDone = progress.completed_steps >= progress.total_steps;
  const stages = template.stages.map((stage) => {
    const p = progress.stages.find((s) => s.key === stage.key);
    return {
      id: stage.key,
      title: text(stage.title),
      progress: p && p.total > 0 ? (p.completed / p.total) * 100 : 0,
    };
  });
  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="primary">{t(`industries.${project.industry}`)}</Badge>
          {project.archived && <Badge tone="warning">{t('projects.archived')}</Badge>}
        </div>
        <h1 className="font-display text-3xl font-semibold text-balance text-fg">
          {project.title}
        </h1>
        {project.tagline && <p className="text-lg text-fg-muted">{project.tagline}</p>}
      </header>

      <Card className="overflow-hidden">
        <div className="grid grid-cols-1 gap-6 p-6 sm:grid-cols-[auto_1fr] sm:items-center">
          <ProgressRing value={progress.percent} size={96} stroke={8} label={t('nav.progress')}>
            <span className="text-lg">{progress.percent}%</span>
          </ProgressRing>
          <div className="flex flex-col items-start gap-3">
            <p className="text-sm text-fg-muted">
              {t('nav.done', { done: progress.completed_steps, total: progress.total_steps })}
            </p>
            {current && !allDone && (
              <Button asChild className="h-auto min-h-10 py-2 text-left whitespace-normal">
                <Link
                  to="/app/projects/$projectId/steps/$stepKey"
                  params={{ projectId: project.id, stepKey: current.key }}
                >
                  {t('overview.continue', { number: current.number, step: text(current.title) })}
                  <ArrowRight aria-hidden />
                </Link>
              </Button>
            )}
            {allDone && <Callout tone="success">{t('overview.allDone')}</Callout>}
            {progress.streak_days > 0 && (
              <span className="flex items-center gap-1.5 text-sm font-medium text-gold-soft-fg">
                <Flame className="size-4" aria-hidden />
                {t('nav.streak', { count: progress.streak_days })}
              </span>
            )}
          </div>
        </div>
        <div className="border-t border-border bg-surface-raised px-6 py-4">
          <StageTrack stages={stages} label={t('overview.stages')} />
        </div>
      </Card>

      <EvaluationTeaser project={project} manages={manages} />

      <section aria-labelledby="badges" className="flex flex-col gap-3">
        <h2 id="badges" className="font-display text-lg font-semibold text-fg">
          {t('badges.title')}
        </h2>
        <BadgeList earned={progress.badges} />
      </section>

      <section aria-labelledby="team" className="flex flex-col gap-3">
        <h2 id="team" className="font-display text-lg font-semibold text-fg">
          {t('settings.team')}
        </h2>
        <ul className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          {project.members.map((member) => (
            <li
              key={member.user.id}
              className="flex items-center gap-3 rounded-lg border border-border p-2.5"
            >
              <Avatar name={member.user.full_name} src={member.user.avatar_url} size="sm" />
              <span className="flex min-w-0 flex-col">
                <TruncatedText className="text-sm font-medium text-fg">
                  {member.user.full_name}
                </TruncatedText>
                <TruncatedText className="text-xs text-fg-muted">
                  {member.title || t(`settings.role.${member.role}`)}
                </TruncatedText>
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="materials" className="flex flex-col gap-3">
        <h2 id="materials" className="font-display text-lg font-semibold text-fg">
          {t('materials.title')}
        </h2>
        <MaterialsPanel projectId={project.id} canCreate={canEdit(project)} />
      </section>
    </div>
  );
}
