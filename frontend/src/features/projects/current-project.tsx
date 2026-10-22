import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowRight, Flame, Plus, Rocket, UsersRound } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  IdeaIllustration,
  ProgressRing,
  Skeleton,
  StageTrack,
  TruncatedText,
} from '@/design-system';
import { templateQuery } from '@/features/incubator/api';
import { ActivityFeed } from '@/features/incubator/overview';
import { findStep } from '@/features/incubator/template';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import { latestProjectQuery, projectQuery } from './api';

/** Dashboard card: the member's current project and what to do next, or how to start. */
export function CurrentProject() {
  const { t } = useTranslation('incubator');
  const latest = useQuery(latestProjectQuery);
  if (latest.isPending) return <Skeleton className="h-64 rounded-xl" />;
  if (latest.isError) return <Callout tone="danger">{errorMessage(latest.error)}</Callout>;
  if (!latest.data) {
    return (
      <Card className="overflow-hidden">
        <div className="grid grid-cols-1 gap-6 p-6 sm:grid-cols-[1fr_auto] sm:items-center">
          <div className="flex flex-col items-start gap-3">
            <Badge tone="gold" icon={<Rocket />}>
              {t('dashboard.badge')}
            </Badge>
            <h2 className="font-display text-xl font-semibold text-fg">
              {t('dashboard.startTitle')}
            </h2>
            <p className="max-w-lg text-sm leading-relaxed text-fg-muted">
              {t('dashboard.startText')}
            </p>
            <div className="flex flex-wrap gap-2 pt-1">
              <Button asChild size="sm">
                <Link to="/app/projects" search={{ new: true }}>
                  <Plus aria-hidden />
                  {t('projects.createFirst')}
                </Link>
              </Button>
              <Button asChild size="sm" variant="secondary">
                <Link to="/app/team">
                  <UsersRound aria-hidden />
                  {t('dashboard.findTeam')}
                </Link>
              </Button>
            </div>
          </div>
          <IdeaIllustration className="hidden w-40 sm:block" />
        </div>
      </Card>
    );
  }
  return <ProjectCard projectId={latest.data.id} />;
}

function ProjectCard({ projectId }: { projectId: string }) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const project = useQuery(projectQuery(projectId));
  const template = useQuery(templateQuery);
  if (!project.data || !template.data) return <Skeleton className="h-64 rounded-xl" />;
  const data = project.data.data;
  const progress = data.progress;
  const current = findStep(template.data, progress.current_step);
  const allDone = progress.completed_steps >= progress.total_steps;
  const stages = template.data.stages.map((stage) => {
    const p = progress.stages.find((s) => s.key === stage.key);
    return {
      id: stage.key,
      title: text(stage.title),
      progress: p && p.total > 0 ? (p.completed / p.total) * 100 : 0,
    };
  });
  return (
    <>
      <Card className="overflow-hidden">
        <div className="grid grid-cols-1 gap-6 p-6 sm:grid-cols-[1fr_auto] sm:items-center">
          <div className="flex min-w-0 flex-col items-start gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone="primary">{t('dashboard.current')}</Badge>
              {progress.streak_days > 1 && (
                <Badge tone="gold" icon={<Flame />}>
                  {t('nav.streak', { count: progress.streak_days })}
                </Badge>
              )}
            </div>
            <TruncatedText as="h2" lines={2} className="font-display text-xl font-semibold text-fg">
              {data.title}
            </TruncatedText>
            {data.tagline && <p className="text-sm text-fg-muted">{data.tagline}</p>}
            {current && !allDone ? (
              <Button
                asChild
                size="sm"
                className="h-auto min-h-8 py-1.5 text-left whitespace-normal"
              >
                <Link
                  to="/app/projects/$projectId/steps/$stepKey"
                  params={{ projectId, stepKey: current.key }}
                >
                  {t('overview.continue', { number: current.number, step: text(current.title) })}
                  <ArrowRight aria-hidden />
                </Link>
              </Button>
            ) : (
              <Button asChild size="sm" variant="secondary">
                <Link to="/app/projects/$projectId" params={{ projectId }}>
                  {t('dashboard.open')}
                </Link>
              </Button>
            )}
          </div>
          <ProgressRing value={progress.percent} size={88} stroke={7} label={t('nav.progress')}>
            <span className="text-base">{progress.percent}%</span>
          </ProgressRing>
        </div>
        <div className="border-t border-border bg-surface-raised px-6 py-4">
          <StageTrack stages={stages} label={t('overview.stages')} />
        </div>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('dashboard.activity')}</CardTitle>
        </CardHeader>
        <CardContent>
          <ActivityFeed projectId={projectId} template={template.data} limit={6} compact />
        </CardContent>
      </Card>
    </>
  );
}
