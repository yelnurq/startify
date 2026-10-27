import { Link } from '@tanstack/react-router';
import {
  BarChart3,
  FolderKanban,
  GraduationCap,
  LayoutDashboard,
  LogIn,
  PanelsTopLeft,
  Rocket,
  Star,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Badge,
  Breadcrumbs,
  Button,
  Card,
  EdgeIllustration,
  EmptyState,
  type IllustrationName,
  illustrations,
  RadarChart,
  SegmentedControl,
  SkipLink,
  StageTrack,
  StatTile,
  type StepItem,
  Stepper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { leaderboard, trends } from '../sample-data';
import { Section, Specimen } from '../section';

import { GaugeSpecimen } from './behaviour';

export function NavigationSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const [period, setPeriod] = useState<'week' | 'month' | 'quarter'>('month');
  const [selected, setSelected] = useState('audience');
  const steps: StepItem[] = [
    {
      id: 'problem',
      title: t('navigation.steps.problem'),
      status: 'done',
      meta: t('navigation.stepMeta.done'),
    },
    {
      id: 'audience',
      title: t('navigation.steps.audience'),
      status: 'current',
      meta: t('navigation.stepMeta.current'),
    },
    { id: 'journey', title: t('navigation.steps.journey'), status: 'todo' },
    {
      id: 'solution',
      title: t('navigation.steps.solution'),
      status: 'locked',
      meta: t('navigation.stepMeta.locked'),
    },
  ];
  return (
    <Section
      id="navigation"
      index={index}
      title={t('navigation.title')}
      lead={t('navigation.lead')}
    >
      <Specimen label={t('navigation.tabs')}>
        <Tabs defaultValue="overview">
          <TabsList>
            <TabsTrigger value="overview" icon={<LayoutDashboard />}>
              {t('navigation.tabOverview')}
            </TabsTrigger>
            <TabsTrigger value="steps" icon={<FolderKanban />} count={14}>
              {t('navigation.tabSteps')}
            </TabsTrigger>
            <TabsTrigger value="team" icon={<Users />} count={4}>
              {t('navigation.tabTeam')}
            </TabsTrigger>
            <TabsTrigger value="evaluations" icon={<Star />}>
              {t('navigation.tabEvaluations')}
            </TabsTrigger>
          </TabsList>
          {(['overview', 'steps', 'team', 'evaluations'] as const).map((tab) => (
            <TabsContent key={tab} value={tab}>
              <p className="text-sm text-fg-muted">{t(`navigation.tabPanels.${tab}`)}</p>
            </TabsContent>
          ))}
        </Tabs>
      </Specimen>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('navigation.stepper')}>
          <Stepper
            steps={steps}
            label={t('navigation.stepper')}
            selectedId={selected}
            onSelect={setSelected}
          />
        </Specimen>
        <div className="flex flex-col gap-4">
          <Specimen label={t('navigation.segmented')} bodyClassName="flex flex-col gap-4">
            <SegmentedControl
              label={t('navigation.period')}
              value={period}
              onValueChange={setPeriod}
              options={[
                { value: 'week', label: t('navigation.week') },
                { value: 'month', label: t('navigation.month') },
                { value: 'quarter', label: t('navigation.quarter') },
              ]}
            />
          </Specimen>
          <Specimen label={t('navigation.breadcrumbs')}>
            <Breadcrumbs
              items={[
                {
                  label: t('navigation.crumbProjects'),
                  render: (c) => <Link to="/design">{c}</Link>,
                },
                { label: 'AgroSense', render: (c) => <Link to="/design">{c}</Link> },
                { label: t('navigation.stages.s1'), render: (c) => <Link to="/design">{c}</Link> },
                { label: t('navigation.steps.audience') },
              ]}
            />
          </Specimen>
          <Specimen label={t('navigation.stageTrack')}>
            <StageTrack
              label={t('navigation.stageTrack')}
              stages={[
                { id: 's1', title: t('navigation.stages.s1'), progress: 100 },
                { id: 's2', title: t('navigation.stages.s2'), progress: 62 },
                { id: 's3', title: t('navigation.stages.s3'), progress: 10 },
                { id: 's4', title: t('navigation.stages.s4'), progress: 0 },
              ]}
            />
          </Specimen>
        </div>
      </div>
      <Specimen label={t('navigation.skipLink')} bodyClassName="flex flex-col items-start gap-3">
        <p className="max-w-2xl text-sm text-fg-muted">{t('navigation.skipLinkHint')}</p>
        {/* Shown in place here; on a page it hides until the keyboard reaches it. */}
        <SkipLink href="#navigation" className="not-sr-only focus:static">
          {t('navigation.skipLinkDemo')}
        </SkipLink>
      </Specimen>
      <Specimen label={t('navigation.accordion')}>
        <Accordion type="single" collapsible defaultValue="q1">
          {(['q1', 'q2', 'q3'] as const).map((q) => (
            <AccordionItem key={q} value={q}>
              <AccordionTrigger>{t(`navigation.faq.${q}.question`)}</AccordionTrigger>
              <AccordionContent>{t(`navigation.faq.${q}.answer`)}</AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </Specimen>
    </Section>
  );
}

const RADAR_AXES = [
  'problem',
  'market',
  'solution',
  'business',
  'team',
  'validation',
  'feasibility',
  'impact',
] as const;

export function DataSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const format = useFormatters();
  const int = (v: number) => format.number(Math.round(v));
  return (
    <Section id="data" index={index} title={t('data.title')} lead={t('data.lead')}>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatTile
          label={t('data.kpi.activeProjects')}
          value={118}
          format={int}
          delta={0.13}
          trend={trends.projects}
          icon={<Rocket />}
          footnote={t('data.kpi.vsMonth')}
        />
        <StatTile
          label={t('data.kpi.students')}
          value={371}
          format={int}
          delta={0.08}
          trend={trends.students}
          icon={<GraduationCap />}
          tone="info"
          footnote={t('data.kpi.vsMonth')}
        />
        <StatTile
          label={t('data.kpi.showcase')}
          value={33}
          format={int}
          delta={0.14}
          trend={trends.showcase}
          icon={<PanelsTopLeft />}
          tone="success"
          footnote={t('data.kpi.vsMonth')}
        />
        <StatTile
          label={t('data.kpi.avgScore')}
          value={63.4}
          format={(v) => format.number(v, { minimumFractionDigits: 1, maximumFractionDigits: 1 })}
          delta={-0.02}
          trend={trends.score}
          icon={<BarChart3 />}
          tone="gold"
          footnote={t('data.kpi.vsMonth')}
        />
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card className="items-center justify-center gap-3 p-6">
          <p className="self-start text-sm font-medium text-fg-muted">{t('data.gauge')}</p>
          <GaugeSpecimen />
          <Badge tone="primary">{t('data.gaugeDelta')}</Badge>
        </Card>
        <Card className="gap-3 p-6">
          <p className="text-sm font-medium text-fg-muted">{t('data.radar.title')}</p>
          <RadarChart
            label={t('data.radar.label')}
            axes={RADAR_AXES.map((key) => ({ key, label: t(`data.radar.axes.${key}`) }))}
            series={[
              { key: 'now', label: t('data.radar.now'), values: [8, 6, 7, 5, 8, 4, 6, 7] },
              {
                key: 'before',
                label: t('data.radar.before'),
                values: [5, 4, 6, 3, 7, 2, 5, 6],
                tone: 'muted',
              },
            ]}
          />
        </Card>
      </div>
      <div className="grid grid-cols-1 gap-4">
        <Table label={t('data.table')}>
          <caption className="sr-only">{t('data.table')}</caption>
          <TableHeader>
            <TableRow>
              <TableHead className="w-16">{t('data.columns.rank')}</TableHead>
              <TableHead>{t('data.columns.project')}</TableHead>
              <TableHead>{t('data.columns.faculty')}</TableHead>
              <TableHead numeric>{t('data.columns.score')}</TableHead>
              <TableHead numeric>{t('data.columns.votes')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {leaderboard.map((row) => (
              <TableRow key={row.project}>
                <TableCell>
                  <span
                    className={
                      row.rank === 1
                        ? 'grid size-7 place-items-center rounded-full bg-gold font-semibold text-gold-fg'
                        : 'grid size-7 place-items-center rounded-full bg-surface-sunken font-semibold text-fg-muted'
                    }
                  >
                    {row.rank}
                  </span>
                </TableCell>
                <TableCell className="font-medium">{row.project}</TableCell>
                <TableCell className="text-fg-muted">
                  {t(`data.faculties.${row.faculty}`)}
                </TableCell>
                <TableCell numeric className="font-semibold">
                  {format.number(row.score, { minimumFractionDigits: 1 })}
                </TableCell>
                <TableCell numeric className="text-fg-muted">
                  {format.number(row.votes)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </Section>
  );
}

const ILLUSTRATIONS = Object.keys(illustrations) as IllustrationName[];

export function EmptyStatesSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  return (
    <Section id="empty" index={index} title={t('empty.title')} lead={t('empty.lead')}>
      <Card>
        <EmptyState
          illustration="projects"
          title={t('empty.exampleTitle')}
          description={t('empty.exampleDescription')}
          action={<Button>{t('empty.exampleAction')}</Button>}
          secondaryAction={<Button variant="secondary">{t('empty.exampleSecondary')}</Button>}
        />
      </Card>
      <Card className="relative min-h-48 justify-center overflow-hidden p-6 sm:p-8">
        <EdgeIllustration
          name="rocket"
          placement="bottom-right"
          className="w-48 -rotate-12 sm:w-60"
        />
        <div className="flex max-w-md flex-col gap-2">
          <p className="font-semibold text-ink">{t('empty.edgeTitle')}</p>
          <p className="text-sm text-fg-muted">{t('empty.edgeText')}</p>
        </div>
      </Card>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
        {ILLUSTRATIONS.map((name) => {
          const Illustration = illustrations[name];
          return (
            <Card key={name} className="items-center gap-2 p-4">
              <Illustration />
              <span className="text-xs font-medium text-fg-muted">{t(`empty.names.${name}`)}</span>
            </Card>
          );
        })}
      </div>
    </Section>
  );
}

export function LayoutsSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const demos = [
    { to: '/design/shell', key: 'shell', icon: LayoutDashboard },
    { to: '/design/workspace', key: 'workspace', icon: FolderKanban },
    { to: '/design/auth', key: 'auth', icon: LogIn },
  ] as const;
  return (
    <Section id="layouts" index={index} title={t('layouts.title')} lead={t('layouts.lead')}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        {demos.map(({ to, key, icon: Icon }) => (
          <Card key={key} interactive className="gap-3 p-5">
            <span className="grid size-10 place-items-center rounded-lg bg-primary-soft text-primary-soft-fg">
              <Icon className="size-5" aria-hidden />
            </span>
            <h3 className="font-semibold text-ink">{t(`layouts.${key}`)}</h3>
            <p className="flex-1 text-sm text-fg-muted">{t(`layouts.${key}Description`)}</p>
            <Button asChild variant="secondary" size="sm" className="self-start">
              <Link to={to}>{t('layouts.open')}</Link>
            </Button>
          </Card>
        ))}
      </div>
    </Section>
  );
}
