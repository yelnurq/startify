import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { Activity, Eye, History, Settings2, Sparkles } from 'lucide-react';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { WorkspaceLayout } from '@/app/layouts/workspace-layout';
import {
  AvatarGroup,
  Breadcrumbs,
  Button,
  Callout,
  EmptyState,
  IconButton,
  Skeleton,
  SkeletonText,
  TruncatedText,
} from '@/design-system';
import { hasRole } from '@/features/auth/api';
import { EvaluationHistory, EvaluationSection } from '@/features/evaluation/evaluation-page';
import { ProjectSettingsSheet } from '@/features/projects/settings-sheet';
import { projectQuery } from '@/features/projects/api';
import { ShowcaseAside, ShowcaseEditor } from '@/features/showcase/editor';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useDocumentMeta } from '@/shared/hooks/use-document-meta';
import { useLocalized } from '@/shared/i18n/localized';

import { stepsQuery, templateQuery } from './api';
import { usePresence } from './hooks';
import { MentorPanel } from './mentor-panel';
import { ActivityFeed, ProjectOverview } from './overview';
import { SaveIndicator } from './save-status';
import type { SaveStatus } from './step-editor';
import { StepNav } from './step-nav';
import { StepPage, type StepBridge } from './step-page';
import { findStep } from './template';

const appRoute = getRouteApi('/app');

interface WorkspaceProps {
  projectId: string;
  stepKey?: string;
  /** The AI evaluation (a specific one with `evaluationId`) or the showcase page instead of the overview. */
  view?: 'evaluation' | 'showcase';
  evaluationId?: string;
}

/** The project workspace: steps on the left, the overview, a step or the evaluation, the mentor. */
export function Workspace({ projectId, stepKey, view, evaluationId }: WorkspaceProps) {
  const { t } = useTranslation('incubator');
  const { t: te } = useTranslation('evaluation');
  const { t: ts } = useTranslation('showcase');
  const { t: tc } = useTranslation();
  const text = useLocalized();
  const { session } = appRoute.useRouteContext();
  const template = useQuery(templateQuery);
  const project = useQuery(projectQuery(projectId));
  const steps = useQuery(stepsQuery(projectId));
  // Presence only for a project that opened: a missing one is not polled.
  const presence = usePresence(projectId, stepKey, project.isSuccess);
  const [save, setSave] = useState<{ status: SaveStatus; savedAt: number | undefined }>({
    status: 'idle',
    savedAt: undefined,
  });
  const [bridge, setBridge] = useState<StepBridge | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const onStatus = useCallback((status: SaveStatus, savedAt: number | undefined) => {
    setSave({ status, savedAt });
  }, []);
  const onBridge = useCallback((value: StepBridge | null) => {
    setBridge(value);
  }, []);

  // The tab names the project and where in it the member is.
  const projectTitle = project.data?.data.title;
  const stepTitle = stepKey && template.data ? findStep(template.data, stepKey)?.title : undefined;
  const where = stepTitle
    ? text(stepTitle)
    : view === 'evaluation'
      ? te('title')
      : view === 'showcase'
        ? ts('editor.title')
        : undefined;
  useDocumentMeta(
    projectTitle
      ? tc('pageTitles.format', {
          page: where
            ? tc('pageTitles.project', { project: projectTitle, page: where })
            : projectTitle,
        })
      : undefined,
  );

  if (project.isError) {
    const missing = project.error instanceof ApiError && project.error.status === 404;
    return (
      <div className="px-4 py-10">
        {missing ? (
          <EmptyState
            illustration="search"
            title={t('workspace.notFound')}
            description={t('workspace.notFoundText')}
            action={
              <Button asChild>
                <Link to="/app/projects">{t('projects.title')}</Link>
              </Button>
            }
          />
        ) : (
          <Callout tone="danger">{errorMessage(project.error)}</Callout>
        )}
      </div>
    );
  }
  if (template.isError) {
    return (
      <div className="px-4 py-10">
        <Callout tone="danger">{errorMessage(template.error)}</Callout>
      </div>
    );
  }
  if (!project.data || !template.data) {
    return (
      <div className="flex flex-col gap-6 px-4 py-8 lg:px-10" aria-busy>
        <Skeleton className="h-8 w-72" />
        <SkeletonText lines={4} />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    );
  }

  const data = project.data.data;
  const step = stepKey ? findStep(template.data, stepKey) : undefined;
  const viewers = (presence?.viewers ?? []).filter((v) => v.user.id !== session.user.id);
  const innovationManager = hasRole(session, 'innovation_manager');
  const manages = innovationManager || hasRole(session, 'tenant_admin');
  const evaluating = view === 'evaluation';
  const showcasing = view === 'showcase';
  const section = evaluating ? te('title') : showcasing ? ts('editor.title') : undefined;

  return (
    <>
      <WorkspaceLayout
        header={
          <div className="flex items-center justify-between gap-3 sm:gap-4">
            {/* Phones show only where the member is (BACKLOG B-004). */}
            <TruncatedText className="min-w-0 flex-1 text-sm font-medium text-fg sm:hidden">
              {step ? text(step.title) : (section ?? data.title)}
            </TruncatedText>
            <Breadcrumbs
              className="hidden min-w-0 sm:block"
              items={[
                {
                  label: t('projects.title'),
                  render: (c) => <Link to="/app/projects">{c}</Link>,
                },
                {
                  label: data.title,
                  render:
                    step || section
                      ? (c) => (
                          <Link to="/app/projects/$projectId" params={{ projectId }}>
                            {c}
                          </Link>
                        )
                      : undefined,
                },
                ...(step ? [{ label: text(step.title) }] : []),
                ...(section ? [{ label: section }] : []),
              ]}
            />
            <div className="flex shrink-0 items-center gap-3">
              {viewers.length > 0 && (
                <div className="flex items-center gap-2">
                  <span className="sr-only">{t('workspace.here')}</span>
                  <AvatarGroup
                    size="xs"
                    max={4}
                    people={viewers.map((v) => ({
                      name: v.user.full_name,
                      src: v.user.avatar_url,
                      online: true,
                    }))}
                  />
                </div>
              )}
              {stepKey && <SaveIndicator status={save.status} savedAt={save.savedAt} />}
              <IconButton
                size="sm"
                label={t('settings.open')}
                icon={<Settings2 />}
                onClick={() => {
                  setSettingsOpen(true);
                }}
              />
            </div>
          </div>
        }
        nav={
          <StepNav
            projectId={projectId}
            template={template.data}
            steps={steps.data}
            activeKey={stepKey}
            activeView={
              evaluating ? 'evaluation' : showcasing ? 'showcase' : stepKey ? 'step' : 'overview'
            }
            evaluation={data.evaluation}
          />
        }
        navTitle={t('nav.steps')}
        aside={
          step ? (
            <MentorPanel
              key={step.key}
              projectId={projectId}
              step={step}
              onApply={bridge ? bridge.apply : null}
            />
          ) : evaluating ? (
            <EvaluationHistory projectId={projectId} activeId={evaluationId} />
          ) : showcasing ? (
            <ShowcaseAside projectId={projectId} />
          ) : (
            <div className="h-full overflow-y-auto p-3">
              <ActivityFeed projectId={projectId} template={template.data} limit={20} compact />
            </div>
          )
        }
        asideTitle={
          step
            ? t('mentor.title')
            : evaluating
              ? te('history.title')
              : showcasing
                ? ts('editor.preview.title')
                : t('overview.activity')
        }
        asideIcon={
          step ? <Sparkles /> : evaluating ? <History /> : showcasing ? <Eye /> : <Activity />
        }
      >
        {stepKey ? (
          <StepPage
            key={stepKey}
            project={data}
            template={template.data}
            stepKey={stepKey}
            steps={steps.data}
            meId={session.user.id}
            innovationManager={innovationManager}
            latestVersion={presence?.versions[stepKey]}
            onStatus={onStatus}
            onBridge={onBridge}
          />
        ) : evaluating ? (
          <EvaluationSection project={data} evaluationId={evaluationId} manages={manages} />
        ) : showcasing ? (
          <ShowcaseEditor project={data} />
        ) : (
          <ProjectOverview project={data} template={template.data} manages={manages} />
        )}
      </WorkspaceLayout>
      <ProjectSettingsSheet
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        project={data}
        etag={project.data.etag}
        meId={session.user.id}
        innovationManager={innovationManager}
      />
    </>
  );
}

const overviewRoute = getRouteApi('/app/projects/$projectId/');
const evaluationRoute = getRouteApi('/app/projects/$projectId/evaluation');
const evaluationHistoryRoute = getRouteApi('/app/projects/$projectId/evaluations/$evaluationId');
const stepRoute = getRouteApi('/app/projects/$projectId/steps/$stepKey');
const showcaseRoute = getRouteApi('/app/projects/$projectId/showcase');

export function ProjectOverviewRoute() {
  const { projectId } = overviewRoute.useParams();
  return <Workspace projectId={projectId} />;
}

export function ProjectStepRoute() {
  const { projectId, stepKey } = stepRoute.useParams();
  return <Workspace projectId={projectId} stepKey={stepKey} />;
}

export function ProjectEvaluationRoute() {
  const { projectId } = evaluationRoute.useParams();
  return <Workspace projectId={projectId} view="evaluation" />;
}

export function ProjectShowcaseRoute() {
  const { projectId } = showcaseRoute.useParams();
  return <Workspace projectId={projectId} view="showcase" />;
}

export function ProjectEvaluationHistoryRoute() {
  const { projectId, evaluationId } = evaluationHistoryRoute.useParams();
  return <Workspace projectId={projectId} view="evaluation" evaluationId={evaluationId} />;
}
