import {
  BadgeCheck,
  Eye,
  Lightbulb,
  Rocket,
  Sparkles,
  Sprout,
  Trophy,
  Truck,
  Users,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  AvatarGroup,
  Badge,
  Button,
  Card,
  CardContent,
  CardFooter,
  cn,
  Progress,
  Tag,
  TruncatedText,
} from '@/design-system';

import { people, sampleProjects } from '../sample-data';
import { Section, Specimen } from '../section';

const SKILLS = ['Python', 'Figma', 'Go', 'Customer Development'];
const FILTERS = ['agro', 'edtech', 'fintech', 'seekingTeam'] as const;

export function IdentitySection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const [filters, setFilters] = useState<string[]>(['agro']);
  const [skills, setSkills] = useState(SKILLS);
  return (
    <Section id="identity" index={index} title={t('identity.title')} lead={t('identity.lead')}>
      <Specimen label={t('identity.badges')} bodyClassName="flex flex-col gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="neutral" icon={<Lightbulb />}>
            {t('identity.stageIdea')}
          </Badge>
          <Badge tone="info">{t('identity.stageValidated')}</Badge>
          <Badge tone="primary" icon={<Rocket />}>
            {t('identity.stageMvp')}
          </Badge>
          <Badge tone="success" icon={<BadgeCheck />}>
            {t('identity.stageInvest')}
          </Badge>
          <Badge tone="gold" icon={<Trophy />}>
            {t('identity.winner')}
          </Badge>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="success" dot>
            {t('identity.published')}
          </Badge>
          <Badge tone="warning" dot>
            {t('identity.review')}
          </Badge>
          <Badge tone="danger" dot>
            {t('identity.rejected')}
          </Badge>
          <Badge tone="primary" size="lg" icon={<Sparkles />}>
            {t('identity.aiScore', { value: 78 })}
          </Badge>
          <Badge tone="neutral" size="sm" icon={<Eye />}>
            {t('identity.views', { count: 1240 })}
          </Badge>
        </div>
      </Specimen>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('identity.tags')} bodyClassName="flex flex-col gap-5">
          <div className="flex flex-wrap gap-2">
            {FILTERS.map((key) => (
              <Tag
                key={key}
                selected={filters.includes(key)}
                onToggle={() => {
                  setFilters((f) => (f.includes(key) ? f.filter((x) => x !== key) : [...f, key]));
                }}
              >
                {t(`identity.filters.${key}`)}
              </Tag>
            ))}
          </div>
          <div className="flex flex-wrap gap-2">
            {skills.map((skill) => (
              <Tag
                key={skill}
                removeLabel={t('identity.removeSkill', { skill })}
                onRemove={() => {
                  setSkills((s) => s.filter((x) => x !== skill));
                }}
              >
                {skill}
              </Tag>
            ))}
            {skills.length < SKILLS.length && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setSkills(SKILLS);
                }}
              >
                {t('identity.resetSkills')}
              </Button>
            )}
          </div>
        </Specimen>
        <Specimen label={t('identity.avatars')} bodyClassName="flex flex-col gap-5">
          <div className="flex flex-wrap items-end gap-3">
            <Avatar name={people[0]?.name ?? ''} size="xl" online />
            <Avatar name={people[1]?.name ?? ''} size="lg" online />
            <Avatar name={people[2]?.name ?? ''} size="md" />
            <Avatar name={people[3]?.name ?? ''} size="sm" />
            <Avatar name={people[4]?.name ?? ''} size="xs" />
          </div>
          <div className="flex items-center justify-between gap-4 rounded-lg bg-surface-raised px-4 py-3">
            <span className="text-sm font-medium text-fg">{t('identity.team')}</span>
            <AvatarGroup people={people} max={4} />
          </div>
        </Specimen>
      </div>
    </Section>
  );
}

const PROJECT_ICONS = { agro: Sprout, tildes: Lightbulb, cargo: Truck } as const;

export function CardsSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  return (
    <Section id="cards" index={index} title={t('cards.title')} lead={t('cards.lead')}>
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {sampleProjects.map((project) => {
          const Icon = PROJECT_ICONS[project.id];
          return (
            <Card key={project.id} interactive className="overflow-hidden">
              <div
                className={cn(
                  'relative flex aspect-[16/9] items-end bg-linear-to-br p-4',
                  project.cover,
                )}
              >
                <div
                  aria-hidden
                  className="absolute inset-0 ornament opacity-15 [--qd-brand:#fff]"
                />
                <span className="relative grid size-11 place-items-center rounded-xl bg-white/90 text-neutral-900 shadow-md backdrop-blur">
                  <Icon className="size-5" aria-hidden />
                </span>
                <Badge
                  tone="neutral"
                  className="absolute top-3 right-3 border-white/40 bg-white/85 text-neutral-800"
                >
                  {t(`cards.projects.${project.id}.industry`)}
                </Badge>
              </div>
              <CardContent className="flex flex-1 flex-col gap-3">
                <div className="flex flex-col gap-1">
                  <h3 className="font-display text-lg font-semibold text-fg">{project.name}</h3>
                  <TruncatedText lines={2} className="text-sm text-fg-muted">
                    {t(`cards.projects.${project.id}.tagline`)}
                  </TruncatedText>
                </div>
                <div className="mt-auto flex flex-col gap-1.5 pt-2">
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-fg-subtle">{t('cards.readiness')}</span>
                    <span className="font-semibold text-fg tabular-nums">{project.score}</span>
                  </div>
                  <Progress value={project.score} size="sm" label={t('cards.readiness')} />
                </div>
              </CardContent>
              <CardFooter className="justify-between">
                <AvatarGroup people={project.team.map((name) => ({ name }))} max={3} size="xs" />
                <span className="flex items-center gap-1.5 text-xs text-fg-subtle">
                  <Users className="size-3.5" aria-hidden />
                  {t(`cards.projects.${project.id}.stage`)}
                </span>
              </CardFooter>
            </Card>
          );
        })}
      </div>
    </Section>
  );
}
