import { Link } from '@tanstack/react-router';
import {
  ArrowLeft,
  Bot,
  Check,
  CheckCircle2,
  MessageSquarePlus,
  SendHorizontal,
  Sparkles,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { WorkspaceLayout } from '@/app/layouts/workspace-layout';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Avatar,
  AvatarGroup,
  Breadcrumbs,
  Button,
  Callout,
  Field,
  IconButton,
  Input,
  LanguageSwitcher,
  LogoMark,
  Progress,
  SegmentedControl,
  Stepper,
  Textarea,
  ThemeToggle,
  toast,
  type StepItem,
} from '@/design-system';

import { people } from '../sample-data';

function StageNav() {
  const { t } = useTranslation('design');
  const [selected, setSelected] = useState('audience');
  const stage1: StepItem[] = [
    { id: 'problem', title: t('navigation.steps.problem'), status: 'done' },
    { id: 'audience', title: t('navigation.steps.audience'), status: 'current' },
    { id: 'journey', title: t('navigation.steps.journey'), status: 'todo' },
    { id: 'solution', title: t('navigation.steps.solution'), status: 'todo' },
  ];
  const later = [
    { id: 's2', progress: 0 },
    { id: 's3', progress: 0 },
    { id: 's4', progress: 0 },
  ] as const;
  return (
    <Accordion type="multiple" defaultValue={['s1']}>
      <AccordionItem value="s1" className="border-0">
        <AccordionTrigger className="px-2 py-3">
          <span className="flex flex-1 flex-col gap-1.5 pr-2">
            <span>{t('navigation.stages.s1')}</span>
            <Progress value={30} size="sm" label={t('navigation.stages.s1')} />
          </span>
        </AccordionTrigger>
        <AccordionContent className="pb-2">
          <Stepper
            steps={stage1}
            label={t('navigation.stages.s1')}
            selectedId={selected}
            onSelect={setSelected}
          />
        </AccordionContent>
      </AccordionItem>
      {later.map((stage) => (
        <AccordionItem key={stage.id} value={stage.id} className="border-0">
          <AccordionTrigger className="px-2 py-3 text-fg-muted">
            <span className="flex flex-1 flex-col gap-1.5 pr-2">
              <span>{t(`navigation.stages.${stage.id}`)}</span>
              <Progress
                value={stage.progress}
                size="sm"
                label={t(`navigation.stages.${stage.id}`)}
              />
            </span>
          </AccordionTrigger>
          <AccordionContent className="px-2">{t('demo.workspace.stageLocked')}</AccordionContent>
        </AccordionItem>
      ))}
    </Accordion>
  );
}

function MentorPanel() {
  const { t } = useTranslation('design');
  const [mode, setMode] = useState<'coach' | 'coauthor'>('coach');
  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-2.5">
        <SegmentedControl
          size="sm"
          label={t('demo.workspace.mode')}
          value={mode}
          onValueChange={setMode}
          options={[
            { value: 'coach', label: t('demo.workspace.coach') },
            { value: 'coauthor', label: t('demo.workspace.coauthor') },
          ]}
        />
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-4" aria-live="polite">
        <div className="flex gap-2.5">
          <span className="grid size-7 shrink-0 place-items-center rounded-full bg-primary-soft text-primary-soft-fg">
            <Bot className="size-4" aria-hidden />
          </span>
          <div className="rounded-2xl rounded-tl-sm bg-surface-raised px-3.5 py-2.5 text-sm leading-relaxed text-fg">
            {t('demo.workspace.mentor1')}
          </div>
        </div>
        <div className="flex flex-row-reverse gap-2.5">
          <Avatar name={people[0]?.name ?? ''} size="xs" />
          <div className="rounded-2xl rounded-tr-sm bg-primary px-3.5 py-2.5 text-sm leading-relaxed text-primary-fg">
            {t('demo.workspace.student1')}
          </div>
        </div>
        <div className="flex gap-2.5">
          <span className="grid size-7 shrink-0 place-items-center rounded-full bg-primary-soft text-primary-soft-fg">
            <Bot className="size-4" aria-hidden />
          </span>
          <div className="flex flex-col gap-2">
            <div className="rounded-2xl rounded-tl-sm bg-surface-raised px-3.5 py-2.5 text-sm leading-relaxed text-fg">
              {t('demo.workspace.mentor2')}
            </div>
            <div className="rounded-xl border border-primary-border bg-primary-soft p-3 text-sm">
              <p className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-primary-soft-fg">
                <Sparkles className="size-3.5" aria-hidden />
                {t('demo.workspace.suggestion')}
              </p>
              <p className="text-fg">{t('demo.workspace.suggestionText')}</p>
              <Button
                size="sm"
                variant="soft"
                className="mt-2.5"
                leftIcon={<Check />}
                onClick={() => {
                  toast.success(t('demo.workspace.applied'));
                }}
              >
                {t('demo.workspace.apply')}
              </Button>
            </div>
          </div>
        </div>
      </div>
      <form
        className="flex items-end gap-2 border-t border-border p-3"
        onSubmit={(e) => {
          e.preventDefault();
        }}
      >
        <Textarea
          rows={1}
          className="min-h-10"
          placeholder={t('demo.workspace.ask')}
          aria-label={t('demo.workspace.ask')}
        />
        <IconButton
          type="submit"
          variant="primary"
          label={t('demo.workspace.send')}
          icon={<SendHorizontal />}
        />
      </form>
    </div>
  );
}

export function WorkspaceDemo() {
  const { t } = useTranslation('design');
  return (
    <div className="min-h-dvh bg-bg">
      <header className="sticky top-0 z-30 flex h-16 items-center gap-3 border-b border-border surface-glass px-4 lg:px-6">
        <Link to="/design/shell" className="rounded-lg" aria-label={t('demo.nav.home')}>
          <LogoMark />
        </Link>
        <Button asChild variant="ghost" size="sm" leftIcon={<ArrowLeft />}>
          <Link to="/design/shell">{t('demo.nav.projects')}</Link>
        </Button>
        <div className="ml-auto flex items-center gap-1">
          <LanguageSwitcher />
          <ThemeToggle />
        </div>
      </header>
      <WorkspaceLayout
        header={
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Breadcrumbs
              items={[
                { label: 'AgroSense', render: (c) => <Link to="/design/shell">{c}</Link> },
                { label: t('navigation.stages.s1') },
                { label: t('navigation.steps.audience') },
              ]}
            />
            <div className="flex items-center gap-4">
              <AvatarGroup people={people.slice(0, 3)} size="xs" />
              <span className="flex items-center gap-1.5 text-xs text-fg-subtle" role="status">
                <CheckCircle2 className="size-3.5 text-success" aria-hidden />
                {t('demo.workspace.saved')}
              </span>
            </div>
          </div>
        }
        nav={<StageNav />}
        navTitle={t('demo.workspace.steps')}
        aside={<MentorPanel />}
        asideTitle={t('demo.workspace.mentor')}
        asideIcon={<Sparkles />}
      >
        <div className="flex flex-col gap-8">
          <header className="flex flex-col gap-2">
            <span className="text-sm font-medium text-primary-text">
              {t('demo.workspace.stepOf', { current: 2, total: 14 })}
            </span>
            <h1 className="font-display text-3xl font-bold text-ink">
              {t('navigation.steps.audience')}
            </h1>
            <p className="text-fg-muted">{t('demo.workspace.stepLead')}</p>
          </header>
          <Callout tone="ai" title={t('demo.workspace.checklistTitle')}>
            <ul className="mt-1 list-disc space-y-1 pl-4">
              <li>{t('demo.workspace.checklist1')}</li>
              <li>{t('demo.workspace.checklist2')}</li>
            </ul>
          </Callout>
          <div className="flex flex-col gap-6">
            <Field
              label={t('demo.workspace.segments')}
              hint={t('demo.workspace.segmentsHint')}
              required
            >
              <Textarea defaultValue={t('demo.workspace.segmentsValue')} rows={4} />
            </Field>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label={t('demo.workspace.personaName')}>
                <Input defaultValue={t('demo.workspace.personaNameValue')} />
              </Field>
              <Field label={t('demo.workspace.personaRole')}>
                <Input defaultValue={t('demo.workspace.personaRoleValue')} />
              </Field>
            </div>
            <Field label={t('demo.workspace.pains')} counter={{ value: 154, max: 600 }}>
              <Textarea defaultValue={t('demo.workspace.painsValue')} rows={3} />
            </Field>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-6">
            <Button variant="ghost" leftIcon={<MessageSquarePlus />}>
              {t('demo.workspace.comment')}
            </Button>
            <Button size="lg" leftIcon={<Check />}>
              {t('demo.workspace.complete')}
            </Button>
          </div>
        </div>
      </WorkspaceLayout>
    </div>
  );
}
