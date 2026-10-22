import { Link } from '@tanstack/react-router';
import {
  ArrowRight,
  Bell,
  CalendarDays,
  ChevronsUpDown,
  Flame,
  FolderKanban,
  Home,
  LineChart,
  LogOut,
  MessageSquareText,
  PanelsTopLeft,
  Settings,
  Sparkles,
  Trophy,
  User,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { AppShell } from '@/app/layouts/app-shell';
import { NavLink, NavSection } from '@/app/layouts/nav-link';
import {
  Avatar,
  Badge,
  Breadcrumbs,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  IconButton,
  LanguageSwitcher,
  ProgressRing,
  StageTrack,
  ThemeToggle,
  TruncatedText,
} from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { people, sampleProjects } from '../sample-data';

const STUDENT = people[0]?.name ?? '';
const CURRENT_PROJECT = sampleProjects[0]?.name ?? '';

export function DemoNav() {
  const { t } = useTranslation('design');
  return (
    <>
      <NavSection>
        <NavLink to="/design/shell" activeOptions={{ exact: true }} icon={<Home />}>
          {t('demo.nav.home')}
        </NavLink>
        <NavLink
          to="/design/workspace"
          icon={<FolderKanban />}
          badge={
            <Badge size="sm" tone="primary">
              {2}
            </Badge>
          }
        >
          {t('demo.nav.projects')}
        </NavLink>
        <NavLink
          to="/design"
          activeOptions={{ exact: true, includeHash: true }}
          icon={<PanelsTopLeft />}
        >
          {t('demo.nav.showcase')}
        </NavLink>
        <NavLink
          to="/design"
          activeOptions={{ exact: true, includeHash: true }}
          hash="cards"
          icon={<Trophy />}
        >
          {t('demo.nav.competitions')}
        </NavLink>
        <NavLink
          to="/design"
          activeOptions={{ exact: true, includeHash: true }}
          hash="overlays"
          icon={<MessageSquareText />}
        >
          {t('demo.nav.mentors')}
        </NavLink>
      </NavSection>
      <NavSection label={t('demo.nav.university')}>
        <NavLink
          to="/design"
          activeOptions={{ exact: true, includeHash: true }}
          hash="data"
          icon={<LineChart />}
        >
          {t('demo.nav.analytics')}
        </NavLink>
        <NavLink
          to="/design"
          activeOptions={{ exact: true, includeHash: true }}
          hash="forms"
          icon={<Settings />}
        >
          {t('demo.nav.settings')}
        </NavLink>
      </NavSection>
    </>
  );
}

export function DemoUser() {
  const { t } = useTranslation('design');
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="flex w-full items-center gap-3 rounded-lg p-1.5 text-left transition-colors hover:bg-surface-raised"
        >
          <Avatar name={STUDENT} size="sm" online />
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-sm font-medium text-fg">{STUDENT}</span>
            <span className="truncate text-xs text-fg-subtle">{t('demo.userRole')}</span>
          </span>
          <ChevronsUpDown className="size-4 text-fg-faint" aria-hidden />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="w-56">
        <DropdownMenuLabel>{STUDENT}</DropdownMenuLabel>
        <DropdownMenuItem icon={<User />}>{t('demo.menu.profile')}</DropdownMenuItem>
        <DropdownMenuItem icon={<Settings />}>{t('demo.menu.settings')}</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem icon={<LogOut />}>{t('demo.menu.signOut')}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function DemoBrand() {
  const { t } = useTranslation(['common', 'design']);
  return (
    <span className="flex min-w-0 flex-col leading-tight">
      <span className="font-display text-base font-bold text-ink">{t('common:app.name')}</span>
      <span className="truncate text-xs text-fg-subtle">{t('design:demo.tenant')}</span>
    </span>
  );
}

export function DemoActions() {
  const { t } = useTranslation('design');
  return (
    <>
      <span className="relative">
        <IconButton label={t('demo.notifications')} icon={<Bell />} />
        <span
          aria-hidden
          className="absolute top-2 right-2 size-2 rounded-full bg-danger-500 ring-2 ring-surface"
        />
      </span>
      <LanguageSwitcher />
      <ThemeToggle />
    </>
  );
}

export function ShellDemo() {
  const { t } = useTranslation('design');
  const format = useFormatters();
  const deadlines = [
    { key: 'startup', date: new Date(2026, 10, 15) },
    { key: 'demoDay', date: new Date(2026, 11, 4) },
    { key: 'grant', date: new Date(2027, 0, 20) },
  ] as const;
  return (
    <AppShell
      brand={<DemoBrand />}
      nav={<DemoNav />}
      user={<DemoUser />}
      actions={<DemoActions />}
      topbar={<Breadcrumbs items={[{ label: t('demo.nav.home') }]} />}
    >
      <div className="flex flex-col gap-8">
        <header className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-col gap-1">
            <p className="text-sm text-fg-subtle">
              {format.dateTime(new Date(), { dateStyle: 'full' })}
            </p>
            <h1 className="font-display text-3xl font-bold text-ink">
              {t('demo.greeting', { name: 'Айгерім' })}
            </h1>
          </div>
          <Badge tone="gold" size="lg" icon={<Flame />}>
            {t('demo.streak', { count: 7 })}
          </Badge>
        </header>

        <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
          <Card className="overflow-hidden">
            <div className="relative bg-linear-to-br from-primary-500 via-primary-600 to-primary-800 p-6 text-white">
              <div aria-hidden className="absolute inset-0 ornament opacity-10 [--qd-brand:#fff]" />
              <div className="relative flex flex-wrap items-center justify-between gap-6">
                <div className="flex flex-col gap-2">
                  <Badge className="self-start border-white/30 bg-white/15 text-white">
                    {t('demo.currentProject')}
                  </Badge>
                  <h2 className="font-display text-2xl font-bold">{CURRENT_PROJECT}</h2>
                  <p className="max-w-md text-sm text-white/80">
                    {t('cards.projects.agro.tagline')}
                  </p>
                </div>
                <ProgressRing
                  value={46}
                  size={84}
                  stroke={7}
                  label={t('demo.progress')}
                  className="[&_.stroke-brand]:stroke-gold-400 [&_.stroke-surface-sunken]:stroke-white/20 [&_span]:text-sm [&_span]:text-white"
                >
                  {format.number(46)}%
                </ProgressRing>
              </div>
            </div>
            <CardContent className="flex flex-col gap-6">
              <StageTrack
                label={t('navigation.stageTrack')}
                stages={[
                  { id: 's1', title: t('navigation.stages.s1'), progress: 100 },
                  { id: 's2', title: t('navigation.stages.s2'), progress: 75 },
                  { id: 's3', title: t('navigation.stages.s3'), progress: 10 },
                  { id: 's4', title: t('navigation.stages.s4'), progress: 0 },
                ]}
              />
              <div className="flex flex-wrap items-center justify-between gap-4 rounded-xl bg-surface-raised p-4">
                <div className="flex flex-col gap-0.5">
                  <span className="text-xs font-medium text-fg-subtle">{t('demo.nextStep')}</span>
                  <span className="font-medium text-fg">{t('demo.nextStepName')}</span>
                </div>
                <Button asChild rightIcon={<ArrowRight />}>
                  <Link to="/design/workspace">{t('demo.continue')}</Link>
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('demo.deadlines')}</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              {deadlines.map((d) => (
                <div
                  key={d.key}
                  className="flex items-start gap-3 rounded-lg p-2 transition-colors hover:bg-surface-raised"
                >
                  <span className="grid size-10 shrink-0 place-items-center rounded-lg bg-gold-soft text-gold-soft-fg">
                    <CalendarDays className="size-4.5" aria-hidden />
                  </span>
                  <span className="flex min-w-0 flex-col">
                    <TruncatedText className="text-sm font-medium text-fg">
                      {t(`demo.deadlineItems.${d.key}`)}
                    </TruncatedText>
                    <span className="text-xs text-fg-subtle">
                      {format.dateTime(d.date, { dateStyle: 'long' })}
                    </span>
                  </span>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>

        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {(['first', 'second'] as const).map((key) => (
            <Card key={key} className="flex-row items-start gap-4 p-5">
              <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary-soft text-primary-soft-fg">
                <Sparkles className="size-5" aria-hidden />
              </span>
              <div className="flex flex-col gap-1">
                <p className="font-medium text-fg">{t(`demo.recommendations.${key}.title`)}</p>
                <p className="text-sm text-fg-muted">{t(`demo.recommendations.${key}.text`)}</p>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </AppShell>
  );
}
