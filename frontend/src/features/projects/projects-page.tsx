import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router';
import { Plus, Search } from 'lucide-react';
import { useDeferredValue, useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  AvatarGroup,
  Badge,
  Button,
  Callout,
  Card,
  Checkbox,
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  Field,
  Input,
  Progress,
  SegmentedControl,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  SkeletonCard,
  toast,
  TruncatedText,
} from '@/design-system';
import { hasRole, tenantQuery } from '@/features/auth/api';
import { templateQuery } from '@/features/incubator/api';
import { findStep } from '@/features/incubator/template';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useLocalized } from '@/shared/i18n/localized';

import {
  INDUSTRIES,
  PROJECT_LIMITS,
  projectsQuery,
  useCreateProject,
  type Industry,
  type ProjectScope,
  type ProjectSummary,
} from './api';

export interface ProjectsSearch {
  scope?: 'all';
  new?: boolean;
}

const route = getRouteApi('/app/projects/');
const appRoute = getRouteApi('/app');

export function ProjectsRoute() {
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const { session } = appRoute.useRouteContext();
  const manager = hasRole(session, 'innovation_manager', 'tenant_admin', 'leadership');
  return (
    <ProjectsPage
      scope={manager && search.scope === 'all' ? 'all' : 'mine'}
      canSeeAll={manager}
      creating={search.new === true}
      onScope={(scope) => {
        void navigate({
          search: (prev) => ({ ...prev, scope: scope === 'all' ? 'all' : undefined }),
        });
      }}
      onCreating={(open) => {
        void navigate({ search: (prev) => ({ ...prev, new: open ? true : undefined }) });
      }}
    />
  );
}

function ProjectsPage({
  scope,
  canSeeAll,
  creating,
  onScope,
  onCreating,
}: {
  scope: ProjectScope;
  canSeeAll: boolean;
  creating: boolean;
  onScope: (scope: ProjectScope) => void;
  onCreating: (open: boolean) => void;
}) {
  const { t } = useTranslation('incubator');
  const [q, setQ] = useState('');
  const query = useDeferredValue(q.trim());
  const projects = useInfiniteQuery(projectsQuery(scope, query || undefined));
  const items = projects.data?.pages.flatMap((page) => page.items) ?? [];

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="font-display text-3xl font-semibold text-fg">{t('projects.title')}</h1>
          <p className="max-w-2xl text-fg-muted">{t('projects.subtitle')}</p>
        </div>
        <Button
          leftIcon={<Plus />}
          className="self-start sm:self-auto"
          onClick={() => {
            onCreating(true);
          }}
        >
          {t('projects.new')}
        </Button>
      </header>

      {(canSeeAll || items.length > 0 || query) && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          {canSeeAll && (
            <SegmentedControl
              label={t('projects.scope.label')}
              value={scope}
              onValueChange={onScope}
              options={[
                { value: 'mine', label: t('projects.scope.mine') },
                { value: 'all', label: t('projects.scope.all') },
              ]}
            />
          )}
          <div className="sm:ml-auto sm:w-72">
            <Input
              value={q}
              leading={<Search />}
              placeholder={t('projects.search')}
              aria-label={t('projects.search')}
              onChange={(e) => {
                setQ(e.target.value);
              }}
            />
          </div>
        </div>
      )}

      {projects.isPending && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" aria-busy>
          {Array.from({ length: 3 }, (_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      )}
      {projects.isError && <Callout tone="danger">{errorMessage(projects.error)}</Callout>}
      {projects.isSuccess && items.length === 0 && (
        <Card>
          {query ? (
            <EmptyState
              illustration="search"
              title={t('projects.nothing')}
              description={t('projects.nothingText')}
            />
          ) : (
            <EmptyState
              illustration={scope === 'all' ? 'projects' : 'idea'}
              title={scope === 'all' ? t('projects.emptyAll') : t('projects.empty')}
              description={scope === 'all' ? undefined : t('projects.emptyText')}
              action={
                <Button
                  leftIcon={<Plus />}
                  onClick={() => {
                    onCreating(true);
                  }}
                >
                  {t('projects.createFirst')}
                </Button>
              }
            />
          )}
        </Card>
      )}
      {items.length > 0 && (
        <ul className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          {items.map((project) => (
            <li key={project.id} className="min-w-0">
              <ProjectCard project={project} />
            </li>
          ))}
        </ul>
      )}
      {projects.hasNextPage && (
        <Button
          variant="secondary"
          className="self-center"
          loading={projects.isFetchingNextPage}
          onClick={() => void projects.fetchNextPage()}
        >
          {t('projects.more')}
        </Button>
      )}

      <NewProjectDialog open={creating} onOpenChange={onCreating} />
    </div>
  );
}

function ProjectCard({ project }: { project: ProjectSummary }) {
  const { t } = useTranslation('incubator');
  const text = useLocalized();
  const template = useQuery(templateQuery);
  const current = template.data ? findStep(template.data, project.current_step) : undefined;
  const percent = project.total_steps
    ? Math.floor((project.completed_steps / project.total_steps) * 100)
    : 0;
  return (
    <Card interactive className="h-full">
      <Link
        to="/app/projects/$projectId"
        params={{ projectId: project.id }}
        className="flex h-full flex-col gap-4 rounded-xl p-5"
      >
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="primary" size="sm">
            {t(`industries.${project.industry}`)}
          </Badge>
          {project.archived && (
            <Badge tone="warning" size="sm">
              {t('projects.archived')}
            </Badge>
          )}
          {project.access !== 'owner' && project.access !== 'member' && (
            <Badge tone="neutral" size="sm">
              {t(`projects.access.${project.access}`)}
            </Badge>
          )}
        </div>
        <div className="flex min-w-0 flex-col gap-1">
          <TruncatedText as="h2" lines={2} className="font-display text-lg font-semibold text-fg">
            {project.title}
          </TruncatedText>
          {project.tagline && (
            <TruncatedText lines={2} className="text-sm text-fg-muted">
              {project.tagline}
            </TruncatedText>
          )}
        </div>
        <div className="mt-auto flex flex-col gap-2">
          <div className="flex items-center justify-between gap-2 text-xs text-fg-subtle">
            <span>
              {t('projects.progress', {
                done: project.completed_steps,
                total: project.total_steps,
              })}
            </span>
            <span className="tabular-nums">{percent}%</span>
          </div>
          <Progress value={percent} size="sm" label={t('projects.progressLabel')} />
          {current && (
            <TruncatedText className="text-xs text-fg-muted">
              {t('projects.current', { number: current.number, step: text(current.title) })}
            </TruncatedText>
          )}
        </div>
        {project.team.length > 0 && (
          <AvatarGroup
            size="xs"
            people={project.team.map((m) => ({ name: m.full_name, src: m.avatar_url }))}
          />
        )}
      </Link>
    </Card>
  );
}

function NewProjectDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation(['incubator', 'common']);
  const text = useLocalized();
  const navigate = useNavigate();
  const tenant = useQuery(tenantQuery);
  const create = useCreateProject();
  const policy = tenant.data?.ip_policy ?? null;
  const [title, setTitle] = useState('');
  const [tagline, setTagline] = useState('');
  const [industry, setIndustry] = useState<Industry | ''>('');
  const [accepted, setAccepted] = useState(false);
  const [touched, setTouched] = useState(false);

  const errors = {
    title:
      title.trim().length === 0
        ? t('common:validation.required')
        : title.trim().length < PROJECT_LIMITS.title[0]
          ? t('common:validation.minLength', { count: PROJECT_LIMITS.title[0] })
          : undefined,
    industry: industry === '' ? t('common:validation.choose') : undefined,
    policy: policy && !accepted ? t('create.acceptRequired') : undefined,
  };
  const valid = !errors.title && !errors.industry && !errors.policy;
  const serverErrors =
    create.error instanceof ApiError
      ? Object.fromEntries(create.error.fieldErrors.map((e) => [e.field, e.code]))
      : {};

  const reset = () => {
    setTitle('');
    setTagline('');
    setIndustry('');
    setAccepted(false);
    setTouched(false);
    create.reset();
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) reset();
        onOpenChange(v);
      }}
    >
      <DialogContent size="lg">
        <form
          className="contents"
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            setTouched(true);
            if (!valid || industry === '') return;
            create.mutate(
              {
                title: title.trim(),
                ...(tagline.trim() ? { tagline: tagline.trim() } : {}),
                industry,
                ...(policy ? { ip_policy_version: policy.version } : {}),
              },
              {
                onSuccess: (project) => {
                  toast.success(t('create.done'));
                  reset();
                  void navigate({
                    to: '/app/projects/$projectId/steps/$stepKey',
                    params: { projectId: project.id, stepKey: project.progress.current_step },
                  });
                },
              },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle>{t('create.title')}</DialogTitle>
            <DialogDescription>{t('create.description')}</DialogDescription>
          </DialogHeader>
          <DialogBody className="flex flex-col gap-5">
            {create.isError &&
              !(create.error instanceof ApiError && create.error.status === 422) && (
                <Callout tone="danger">{errorMessage(create.error)}</Callout>
              )}
            <Field
              label={t('create.name')}
              required
              error={
                (touched ? errors.title : undefined) ??
                (serverErrors.title ? t('common:validation.invalid') : undefined)
              }
              hint={t('create.nameHint')}
            >
              <Input
                value={title}
                autoFocus
                maxLength={PROJECT_LIMITS.title[1]}
                placeholder={t('create.namePlaceholder')}
                onChange={(e) => {
                  setTitle(e.target.value);
                }}
              />
            </Field>
            <Field
              label={t('create.tagline')}
              optional
              counter={{ value: tagline.length, max: PROJECT_LIMITS.tagline }}
            >
              <Input
                value={tagline}
                maxLength={PROJECT_LIMITS.tagline}
                placeholder={t('create.taglinePlaceholder')}
                onChange={(e) => {
                  setTagline(e.target.value);
                }}
              />
            </Field>
            <Field
              label={t('create.industry')}
              required
              error={touched ? errors.industry : undefined}
            >
              <Select
                value={industry || undefined}
                onValueChange={(v) => {
                  setIndustry(v as Industry);
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('create.industryPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {INDUSTRIES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {t(`industries.${value}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            {policy && (
              <div className="flex flex-col gap-2">
                <p className="text-sm font-medium text-fg">{t('create.policyTitle')}</p>
                <div
                  tabIndex={0}
                  className="max-h-40 overflow-y-auto rounded-lg border border-border bg-surface-raised"
                >
                  <p className="p-3 text-sm leading-relaxed whitespace-pre-line text-fg-muted">
                    {text(policy.text)}
                  </p>
                </div>
                <label className="flex items-start gap-3 text-sm text-fg">
                  <Checkbox
                    checked={accepted}
                    onCheckedChange={(v) => {
                      setAccepted(v === true);
                    }}
                    aria-invalid={touched && Boolean(errors.policy)}
                  />
                  <span>{t('create.accept')}</span>
                </label>
                {touched && errors.policy && <p className="text-sm text-danger">{errors.policy}</p>}
              </div>
            )}
          </DialogBody>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                reset();
                onOpenChange(false);
              }}
            >
              {t('common:actions.cancel')}
            </Button>
            <Button type="submit" loading={create.isPending}>
              {t('create.submit')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
