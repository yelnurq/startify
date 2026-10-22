import { Link } from '@tanstack/react-router';
import { Eye, Gauge } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { Badge, Button, cn, Skeleton, SkeletonText, TruncatedText } from '@/design-system';
import { levelTone } from '@/features/evaluation/logic';
import type { components } from '@/shared/api/schema.gen';
import { useLocalized } from '@/shared/i18n/localized';

import type { ShowcaseCard, UniversityBrief } from './api';
import { monogram, splitVisible, type Industry } from './logic';

type ReadinessLevel = components['schemas']['ReadinessLevel'];

// Covers without an image get a gradient of the brand family, varied by industry.
const GRADIENTS = [
  'from-primary-500 via-primary-700 to-primary-950',
  'from-info-500 via-primary-700 to-primary-950',
  'from-success-500 via-primary-700 to-primary-950',
  'from-gold-400 via-primary-600 to-primary-950',
] as const;

const INDUSTRY_GRADIENT: Record<Industry, number> = {
  agritech: 2,
  edtech: 0,
  fintech: 3,
  healthtech: 1,
  logistics: 0,
  cleantech: 2,
  govtech: 1,
  foodtech: 3,
  fashion: 3,
  ecommerce: 0,
  manufacturing: 1,
  energy: 3,
  tourism: 2,
  social: 1,
  other: 0,
};

/** The cover image, or a branded gradient with the project's monogram. */
export function CardCover({
  title,
  industry,
  coverUrl,
  priority = false,
  monogram: showMonogram = true,
  className,
}: {
  title: string;
  industry: Industry;
  coverUrl: string | null;
  /** The largest image of the page (LCP): loaded first, not lazily. */
  priority?: boolean;
  /** Initials on the placeholder; off where the title is written over it. */
  monogram?: boolean;
  className?: string;
}) {
  if (coverUrl) {
    return (
      <img
        src={coverUrl}
        alt=""
        width={1600}
        height={900}
        loading={priority ? 'eager' : 'lazy'}
        decoding={priority ? 'sync' : 'async'}
        fetchPriority={priority ? 'high' : 'auto'}
        className={cn('size-full object-cover', className)}
      />
    );
  }
  return (
    <div
      aria-hidden
      className={cn(
        'relative grid size-full place-items-center overflow-hidden bg-linear-to-br',
        GRADIENTS[INDUSTRY_GRADIENT[industry]],
        className,
      )}
    >
      <div className="absolute inset-0 ornament opacity-[0.1]" />
      {showMonogram && (
        <span className="relative font-display text-5xl font-semibold text-white/90">
          {monogram(title)}
        </span>
      )}
    </div>
  );
}

/** Readiness score of the latest AI evaluation. */
export function ScoreBadge({
  score,
  level,
  size = 'md',
  className,
}: {
  score: number | null;
  level: ReadinessLevel | null;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}) {
  const { t } = useTranslation('showcase');
  if (score === null || level === null) return null;
  const label = t('card.score', { score, level: t(`levels.${level}`) });
  return (
    <Badge tone={levelTone(level)} size={size} icon={<Gauge aria-hidden />} className={className}>
      <span className="sr-only">{label}</span>
      <span aria-hidden className="tabular-nums">
        {score}
      </span>
    </Badge>
  );
}

/** The university of a project in the national showcase. */
export function UniversityChip({
  university,
  className,
}: {
  university: UniversityBrief;
  className?: string;
}) {
  const text = useLocalized();
  return (
    <span
      className={cn(
        'inline-flex h-6 max-w-full items-center gap-1.5 rounded-full border border-border bg-surface px-2 text-xs font-medium text-fg shadow-sm',
        className,
      )}
    >
      {university.logo_url ? (
        <img
          src={university.logo_url}
          alt=""
          width={16}
          height={16}
          className="size-4 rounded-sm"
        />
      ) : (
        <span
          aria-hidden
          className="size-2.5 shrink-0 rounded-full"
          style={{ backgroundColor: university.color }}
        />
      )}
      <span className="truncate">{text(university.short_name) || text(university.name)}</span>
    </span>
  );
}

interface ProjectLinkProps {
  card: Pick<ShowcaseCard, 'slug' | 'url'>;
  /** The page lives on another university's site. */
  external: boolean;
  className?: string;
  children: ReactNode;
}

/** A link to a project page: in the app, or to the site of its university. */
export function ProjectLink({ card, external, className, children }: ProjectLinkProps) {
  if (external) {
    return (
      <a href={card.url} className={className}>
        {children}
      </a>
    );
  }
  return (
    <Link to="/showcase/$slug" params={{ slug: card.slug }} className={className}>
      {children}
    </Link>
  );
}

interface ShowcaseCardViewProps {
  card: ShowcaseCard;
  external: boolean;
  /** Show the university (national scope). */
  showUniversity: boolean;
  onQuickView?: () => void;
  /** Covers above the fold load first. */
  priority?: boolean;
  /** False for a preview of a page that is not published yet. */
  linked?: boolean;
}

/** A project in the catalogue: the whole card opens the page; quick view is a separate button. */
export function ShowcaseCardView({
  card,
  external,
  showUniversity,
  onQuickView,
  priority = false,
  linked = true,
}: ShowcaseCardViewProps) {
  const { t } = useTranslation('showcase');
  const text = useLocalized();
  const needs = splitVisible(card.needs, 2);
  const faculty = text(card.faculty);
  return (
    <article className="group relative flex h-full flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-sm transition-[box-shadow,transform,border-color] duration-200 focus-within:border-border-strong hover:-translate-y-0.5 hover:border-border-strong hover:shadow-md">
      <div className="relative aspect-video overflow-hidden bg-surface-sunken">
        <CardCover
          title={card.title}
          industry={card.industry}
          coverUrl={card.cover_url}
          priority={priority}
          className="transition-transform duration-300 group-hover:scale-[1.02]"
        />
        <div className="absolute inset-x-3 top-3 flex items-start justify-between gap-2">
          {showUniversity ? <UniversityChip university={card.university} /> : <span />}
          <ScoreBadge score={card.score} level={card.level} className="shadow-sm" />
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="primary" size="sm">
            {t(`industries.${card.industry}`)}
          </Badge>
          <Badge size="sm">{t(`stages.${card.stage}`)}</Badge>
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="font-display text-lg leading-snug font-semibold text-fg">
            {linked ? (
              <ProjectLink
                card={card}
                external={external}
                className="rounded-sm outline-none after:absolute after:inset-0 after:content-[''] focus-visible:ring-2 focus-visible:ring-primary-500"
              >
                {card.title}
                {external && (
                  <span className="sr-only">
                    {' '}
                    ({t('card.external', { university: text(card.university.short_name) })})
                  </span>
                )}
              </ProjectLink>
            ) : (
              card.title
            )}
          </h3>
          {card.tagline && (
            <TruncatedText as="p" lines={2} className="text-sm leading-relaxed text-fg-muted">
              {card.tagline}
            </TruncatedText>
          )}
        </div>
        <div className="mt-auto flex flex-col gap-3">
          <p className="flex min-w-0 items-center gap-1 text-xs text-fg-subtle">
            {faculty && (
              <>
                <TruncatedText>{faculty}</TruncatedText>
                <span aria-hidden>·</span>
              </>
            )}
            <span className="shrink-0 tabular-nums">{card.year}</span>
          </p>
          <div className="flex items-center justify-between gap-2">
            {card.needs.length > 0 ? (
              <p className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-fg-muted">
                <span className="font-medium">{t('card.seeking')}</span>
                {needs.visible.map((need) => (
                  <Badge key={need} tone="gold" size="sm">
                    {t(`needs.${need}`)}
                  </Badge>
                ))}
                {needs.rest > 0 && (
                  <Badge size="sm">
                    <span aria-hidden>+</span>
                    {needs.rest}
                  </Badge>
                )}
              </p>
            ) : (
              <span />
            )}
            {onQuickView && (
              <Button
                variant="ghost"
                size="icon-sm"
                className="relative z-10 shrink-0"
                aria-label={t('card.quickViewOf', { title: card.title })}
                onClick={onQuickView}
              >
                <Eye aria-hidden />
              </Button>
            )}
          </div>
        </div>
      </div>
    </article>
  );
}

/** Placeholder of a card while the catalogue loads. */
export function ShowcaseCardSkeleton() {
  return (
    <div className="flex flex-col overflow-hidden rounded-xl border border-border bg-surface">
      <Skeleton className="aspect-video rounded-none" />
      <div className="flex flex-col gap-3 p-4">
        <div className="flex gap-1.5">
          <Skeleton className="h-5 w-20 rounded-full" />
          <Skeleton className="h-5 w-14 rounded-full" />
        </div>
        <Skeleton className="h-6 w-3/4" />
        <SkeletonText lines={2} />
      </div>
    </div>
  );
}
