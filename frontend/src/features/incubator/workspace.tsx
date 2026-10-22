import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import { Activity, Settings2, Sparkles } from 'lucide-react';
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
import { ProjectSettingsSheet } from '@/features/projects/settings-sheet';
import { projectQuery } from '@/features/projects/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
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

/** The project workspace: steps on the left, the overview or a step, the mentor. */
export function Workspace({ projectId, stepKey }: { projectId: string; stepKey?: string }) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const { session } = appRoute.useRouteContext();
  const template = useQuery(templateQuery);
  const project = useQuery(projectQuery(projectId));
  const steps = useQuery(stepsQuery(projectId));
  const presence = usePresence(projectId, stepKey);
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

  return (
    <>
      <WorkspaceLayout
        header={
          <div className="flex items-center justify-between gap-3 sm:gap-4">
            {/* Phones show only where the member is (BACKLOG B-004). */}
            <TruncatedText className="min-w-0 flex-1 text-sm font-medium text-fg sm:hidden">
              {step ? text(step.title) : data.title}
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
                  render: step
                    ? (c) => (
                        <Link to="/app/projects/$projectId" params={{ projectId }}>
                          {c}
                        </Link>
                      )
                    : undefined,
                },
                ...(step ? [{ label: text(step.title) }] : []),
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
          ) : (
            <div className="h-full overflow-y-auto p-3">
              <ActivityFeed projectId={projectId} template={template.data} limit={20} compact />
            </div>
          )
        }
        asideTitle={step ? t('mentor.title') : t('overview.activity')}
        asideIcon={step ? <Sparkles /> : <Activity />}
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
        ) : (
          <ProjectOverview project={data} template={template.data} />
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
const stepRoute = getRouteApi('/app/projects/$projectId/steps/$stepKey');

export function ProjectOverviewRoute() {
  const { projectId } = overviewRoute.useParams();
  return <Workspace projectId={projectId} />;
}

export function ProjectStepRoute() {
  const { projectId, stepKey } = stepRoute.useParams();
  return <Workspace projectId={projectId} stepKey={stepKey} />;
}
