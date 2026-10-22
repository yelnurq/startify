import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import {
  ArrowRight,
  BookOpenText,
  Check,
  Circle,
  FileUp,
  Mail,
  MailCheck,
  Sparkles,
  Users,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

import {
  Badge,
  Button,
  Callout,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Illustration,
  ProgressRing,
  Skeleton,
  SkeletonText,
  Tag,
  toast,
  TruncatedText,
} from '@/design-system';
import { hasRole, useResendVerification, type Session } from '@/features/auth/api';
import { meQuery, type Me } from '@/features/profile/api';
import { CurrentProject } from '@/features/projects/current-project';
import { ROLE_ORDER } from '@/features/shell/roles';
import { latestTeamRequestsQuery } from '@/features/team/api';
import { errorMessage } from '@/shared/api/errors';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { profileChecklist } from './completeness';

function partOfDay(hour: number): 'morning' | 'day' | 'evening' | 'night' {
  if (hour >= 5 && hour < 12) return 'morning';
  if (hour >= 12 && hour < 18) return 'day';
  if (hour >= 18 && hour < 23) return 'evening';
  return 'night';
}

/** The first word of a full name, for a friendly greeting. */
function firstName(fullName: string): string {
  return fullName.trim().split(/\s+/)[0] ?? fullName;
}

/** Home of the signed-in area: greeting, profile progress, team requests. */
export function DashboardPage({ session }: { session: Session }) {
  const { t } = useTranslation(['app', 'common']);
  const format = useFormatters();
  const me = useQuery(meQuery);
  const now = new Date();
  const roles = ROLE_ORDER.filter((role) => session.membership.roles.includes(role));
  const admin = hasRole(session, 'tenant_admin');
  const manager = hasRole(session, 'innovation_manager');

  return (
    <div className="flex flex-col gap-8">
      <header className="flex items-center justify-between gap-6 rounded-card border border-border bg-surface px-6 py-5">
        <div className="flex min-w-0 flex-col gap-3">
          <div className="flex flex-col gap-1">
            <p className="text-sm text-fg-subtle first-letter:uppercase">
              {format.dateTime(now, { dateStyle: 'full' })}
            </p>
            <h1 className="font-display text-3xl font-bold text-balance text-ink">
              {t(`dashboard.greeting.${partOfDay(now.getHours())}`, {
                name: firstName(session.user.full_name),
              })}
            </h1>
          </div>
          <ul className="flex flex-wrap gap-2" aria-label={t('dashboard.roles')}>
            {roles.map((role) => (
              <li key={role}>
                <Badge tone={role === 'student' ? 'primary' : 'gold'}>
                  {t(`common:roles.${role}`)}
                </Badge>
              </li>
            ))}
          </ul>
        </div>
        <Illustration name="route" className="hidden w-44 shrink-0 md:block" />
      </header>

      {!session.user.email_verified && <VerifyEmailBanner email={session.user.email} />}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-6 lg:col-span-2">
          <CurrentProject />
          <LatestRequests />
        </div>
        <div className="flex min-w-0 flex-col gap-6">
          {me.isPending && <Skeleton className="h-72 rounded-xl" />}
          {me.isError && <Callout tone="danger">{errorMessage(me.error)}</Callout>}
          {me.isSuccess && (
            <>
              <ProfileProgress me={me.data} />
              <StudySummary me={me.data} />
            </>
          )}
          {(admin || manager) && <AdminShortcuts admin={admin} />}
        </div>
      </div>
    </div>
  );
}

function VerifyEmailBanner({ email }: { email: string }) {
  const { t } = useTranslation(['app', 'auth']);
  const resend = useResendVerification();
  return (
    <Callout
      tone="warning"
      title={t('dashboard.verify.title')}
      action={
        <Button
          size="sm"
          variant="secondary"
          leftIcon={resend.isSuccess ? <MailCheck /> : <Mail />}
          loading={resend.isPending}
          disabled={resend.isSuccess}
          onClick={() => {
            resend.mutate(undefined, {
              onError: (err) => {
                toast.error(errorMessage(err));
              },
            });
          }}
        >
          {resend.isSuccess ? t('auth:verify.resent') : t('auth:verify.resend')}
        </Button>
      }
    >
      {t('dashboard.verify.text', { email })}
    </Callout>
  );
}

function LatestRequests() {
  const { t } = useTranslation(['app', 'team']);
  const format = useFormatters();
  const requests = useQuery(latestTeamRequestsQuery);
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-3 pb-2">
        <CardTitle>{t('dashboard.requests.title')}</CardTitle>
        <Button asChild variant="link" size="sm">
          <Link to="/app/team">
            {t('dashboard.requests.all')}
            <ArrowRight aria-hidden />
          </Link>
        </Button>
      </CardHeader>
      <CardContent className="pt-3">
        {requests.isPending && <SkeletonText lines={4} />}
        {requests.isError && <Callout tone="danger">{errorMessage(requests.error)}</Callout>}
        {requests.isSuccess && requests.data.length === 0 && (
          <div className="flex flex-col items-start gap-3 py-2">
            <p className="text-sm text-fg-muted">{t('dashboard.requests.empty')}</p>
            <Button asChild size="sm" variant="soft">
              <Link to="/app/team" search={{ new: true }}>
                {t('dashboard.requests.create')}
              </Link>
            </Button>
          </div>
        )}
        {requests.isSuccess && requests.data.length > 0 && (
          <ul className="flex flex-col divide-y divide-border">
            {requests.data.map((request) => (
              <li key={request.id} className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
                <div className="flex items-start justify-between gap-3">
                  <TruncatedText as="p" lines={2} className="font-medium text-fg">
                    {request.title}
                  </TruncatedText>
                  <span className="shrink-0 text-xs text-fg-subtle">
                    {format.relative(request.created_at)}
                  </span>
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="mr-1 text-xs text-fg-subtle">{request.author.full_name}</span>
                  {request.roles_needed.map((role) => (
                    <Badge key={role} size="sm" tone="primary">
                      {t(`team:roles.${role}`)}
                    </Badge>
                  ))}
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function ProfileProgress({ me }: { me: Me }) {
  const { t } = useTranslation('app');
  const items = profileChecklist(me);
  const done = items.filter((item) => item.done).length;
  const value = Math.round((done / items.length) * 100);
  return (
    <Card>
      <CardContent className="flex flex-col gap-5">
        <div className="flex items-center gap-4">
          <ProgressRing value={value} label={t('dashboard.profile.ring', { value })} size={64}>
            <span className="text-sm font-semibold text-fg tabular-nums">{value}%</span>
          </ProgressRing>
          <div className="flex flex-col gap-0.5">
            <h2 className="font-semibold text-ink">{t('dashboard.profile.title')}</h2>
            <p className="text-sm text-fg-muted">
              {t('dashboard.profile.counter', { done, total: items.length })}
            </p>
          </div>
        </div>
        <ul className="flex flex-col gap-2">
          {items.map((item) => (
            <li key={item.key} className="flex items-center gap-2.5 text-sm">
              {item.done ? (
                <Check className="size-4 shrink-0 text-success" aria-hidden />
              ) : (
                <Circle className="size-4 shrink-0 text-fg-faint" aria-hidden />
              )}
              <span className={item.done ? 'text-fg-subtle line-through' : 'text-fg'}>
                {t(`dashboard.profile.items.${item.key}`)}
              </span>
              <span className="sr-only">
                {item.done ? t('dashboard.profile.done') : t('dashboard.profile.todo')}
              </span>
            </li>
          ))}
        </ul>
        {done < items.length && (
          <Button asChild variant="secondary" size="sm" className="self-start">
            <Link to="/app/profile">{t('dashboard.profile.cta')}</Link>
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

function StudySummary({ me }: { me: Me }) {
  const { t } = useTranslation(['app', 'common']);
  const text = useLocalized();
  const { membership, profile } = me;
  const rows: { key: 'faculty' | 'department' | 'group' | 'level'; value: string }[] = [];
  if (membership.faculty) rows.push({ key: 'faculty', value: text(membership.faculty.name) });
  if (membership.department) {
    rows.push({ key: 'department', value: text(membership.department.name) });
  }
  if (membership.group) rows.push({ key: 'group', value: membership.group.code });
  if (membership.study_level) {
    const level = t(`common:studyLevels.${membership.study_level}`);
    rows.push({
      key: 'level',
      value: membership.course
        ? t('dashboard.study.levelCourse', { level, course: membership.course })
        : level,
    });
  }
  if (rows.length === 0 && profile.interests.length === 0) return null;
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-3">
        <CardTitle>{t('dashboard.study.title')}</CardTitle>
        <Button asChild variant="link" size="sm">
          <Link to="/app/profile">{t('common:actions.edit')}</Link>
        </Button>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {rows.length > 0 && (
          <dl className="grid grid-cols-1 gap-3 text-sm">
            {rows.map((row) => (
              <div key={row.key} className="flex flex-col gap-0.5">
                <dt className="text-xs text-fg-subtle">{t(`dashboard.study.${row.key}`)}</dt>
                <dd className="text-fg">{row.value}</dd>
              </div>
            ))}
          </dl>
        )}
        {profile.interests.length > 0 && (
          <ul className="flex flex-wrap gap-1.5" aria-label={t('dashboard.study.interests')}>
            {profile.interests.map((interest) => (
              <li key={interest}>
                <Tag>{interest}</Tag>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function AdminShortcuts({ admin }: { admin: boolean }) {
  const { t } = useTranslation('app');
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('dashboard.admin.title')}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        <Button asChild variant="secondary" className="justify-start">
          <Link to="/app/admin/members">
            <Users aria-hidden />
            {t('nav.members')}
          </Link>
        </Button>
        {admin && (
          <Button asChild variant="secondary" className="justify-start">
            <Link to="/app/admin/import">
              <FileUp aria-hidden />
              {t('nav.import')}
            </Link>
          </Button>
        )}
        <Button asChild variant="secondary" className="justify-start">
          <Link to="/app/admin/knowledge">
            <BookOpenText aria-hidden />
            {t('nav.knowledge')}
          </Link>
        </Button>
        {admin && (
          <Button asChild variant="secondary" className="justify-start">
            <Link to="/app/admin/ai">
              <Sparkles aria-hidden />
              {t('nav.ai')}
            </Link>
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

const dashboardRoute = getRouteApi('/app/');

export function DashboardRoute() {
  const { session } = dashboardRoute.useRouteContext();
  return <DashboardPage session={session} />;
}
