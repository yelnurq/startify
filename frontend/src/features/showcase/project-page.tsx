import { useQuery } from '@tanstack/react-query';
import { getRouteApi, Link } from '@tanstack/react-router';
import {
  CircleDollarSign,
  Copy,
  FlaskConical,
  Handshake,
  Mail,
  PlayCircle,
  Send,
  Share2,
  UserPlus,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  Avatar,
  Badge,
  Breadcrumbs,
  Button,
  Callout,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  Progress,
  Skeleton,
  SkeletonText,
  toast,
} from '@/design-system';
import { sessionQuery } from '@/features/auth/api';
import { ApiError } from '@/shared/api/client';
import { errorMessage } from '@/shared/api/errors';
import { useDocumentMeta } from '@/shared/hooks/use-document-meta';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

import { showcaseProjectQuery, type ShowcaseProject } from './api';
import { CardCover, ScoreBadge, UniversityChip } from './card';
import { ContactDialog } from './contact-dialog';
import {
  excerpt,
  isWebLink,
  presentSections,
  shareLinks,
  videoEmbed,
  type SectionKey,
  type ShowcaseNeed,
} from './logic';
import { SiteFrame } from './site';

const route = getRouteApi('/showcase/$slug');

const NEED_ICONS: Record<ShowcaseNeed, LucideIcon> = {
  investment: CircleDollarSign,
  team: UserPlus,
  partner: Handshake,
  pilot: FlaskConical,
  mentor: Users,
};

const TEAM_ROLES = new Set(['owner', 'member', 'supervisor']);

/** A published project as a magazine page: hero, video, sections, team and contact. */
export function ProjectPage() {
  const { slug } = route.useParams();
  const project = useQuery(showcaseProjectQuery(slug));
  return (
    <SiteFrame>
      {project.isPending ? (
        <ProjectSkeleton />
      ) : project.isError ? (
        <ProjectError error={project.error} onRetry={() => void project.refetch()} />
      ) : (
        <ProjectView project={project.data} />
      )}
    </SiteFrame>
  );
}

function ProjectView({ project }: { project: ShowcaseProject }) {
  const { t } = useTranslation('showcase');
  const text = useLocalized();
  const { dateTime } = useFormatters();
  const [contactOpen, setContactOpen] = useState(false);
  const { card, sections } = project;
  const university = text(card.university.short_name) || text(card.university.name);
  const present = presentSections({ sections, team: project.team, needs: card.needs });
  const active = useActiveSection(present);
  const video = videoEmbed(project.video_url);
  const faculty = text(card.faculty);

  useDocumentMeta(
    t('meta.projectTitle', { title: card.title, university }),
    excerpt(card.tagline || sections.problem, 160),
  );

  const contact = project.contact_enabled && (
    <Button
      size="lg"
      leftIcon={<Mail />}
      onClick={() => {
        setContactOpen(true);
      }}
    >
      {t('project.contact')}
    </Button>
  );

  return (
    <article aria-labelledby="project-title">
      <div className="mx-auto flex max-w-page flex-col gap-5 px-4 pt-6 sm:px-6">
        <Breadcrumbs
          items={[
            { label: t('site.showcase'), render: (c) => <Link to="/showcase">{c}</Link> },
            { label: card.title },
          ]}
        />
        <header className="relative isolate overflow-hidden rounded-3xl bg-primary-950 shadow-lg">
          <div className="aspect-[4/5] sm:aspect-video lg:aspect-[21/9]">
            <CardCover
              title={card.title}
              industry={card.industry}
              coverUrl={card.cover_url}
              priority
              monogram={false}
            />
          </div>
          <div
            aria-hidden
            className="absolute inset-0 bg-linear-to-t from-neutral-950/90 via-neutral-950/45 to-neutral-950/5"
          />
          <div className="absolute inset-x-0 bottom-0 flex flex-col gap-4 p-5 sm:p-8 lg:p-10">
            <div className="flex flex-wrap items-center gap-2">
              <Badge tone="primary">{t(`industries.${card.industry}`)}</Badge>
              <Badge>{t(`stages.${card.stage}`)}</Badge>
              <ScoreBadge score={card.score} level={card.level} />
            </div>
            <h1
              id="project-title"
              className="max-w-4xl font-display text-3xl leading-tight font-semibold text-balance text-white sm:text-5xl"
            >
              {card.title}
            </h1>
            {card.tagline && (
              <p className="max-w-3xl text-base leading-relaxed text-white/90 sm:text-xl">
                {card.tagline}
              </p>
            )}
          </div>
        </header>
        <div className="flex flex-wrap items-center gap-3">
          {contact}
          <ShareMenu url={card.url} title={card.title} />
          <UniversityChip university={card.university} className="ml-auto h-8 px-3 text-sm" />
        </div>
      </div>

      {present.length > 1 && (
        <nav
          aria-label={t('project.sections.label')}
          className="sticky top-16 z-20 mt-6 border-y border-border/70 surface-glass"
        >
          <ol className="mx-auto flex max-w-page [scrollbar-width:none] gap-1 overflow-x-auto px-4 py-2 sm:px-6">
            {present.map((key) => (
              <li key={key} className="shrink-0">
                <a
                  href={`#${key}`}
                  aria-current={active === key ? 'location' : undefined}
                  className={cn(
                    'flex h-9 items-center rounded-full px-3.5 text-sm font-medium whitespace-nowrap text-fg-muted transition-colors hover:bg-surface-raised hover:text-fg',
                    active === key && 'bg-primary-soft text-primary-soft-fg hover:bg-primary-soft',
                  )}
                >
                  {t(`project.sections.${key}`)}
                </a>
              </li>
            ))}
          </ol>
        </nav>
      )}

      <div className="mx-auto grid max-w-page grid-cols-1 gap-10 px-4 py-10 sm:px-6 lg:grid-cols-[minmax(0,1fr)_20rem] lg:gap-14 lg:py-14">
        <div className="flex min-w-0 flex-col gap-14">
          {video ? (
            <section aria-label={t('project.video.heading')}>
              <div className="aspect-video overflow-hidden rounded-2xl border border-border bg-surface-sunken shadow-sm">
                <iframe
                  src={video.src}
                  title={t('project.video.frame', { title: card.title })}
                  loading="lazy"
                  allow="accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture; fullscreen"
                  allowFullScreen
                  referrerPolicy="strict-origin-when-cross-origin"
                  className="size-full"
                />
              </div>
            </section>
          ) : (
            isWebLink(project.video_url) && (
              <div>
                <Button asChild variant="secondary">
                  <a href={project.video_url} target="_blank" rel="noopener noreferrer">
                    <PlayCircle aria-hidden />
                    {t('project.video.watch')}
                  </a>
                </Button>
              </div>
            )
          )}

          {present.map((key) => (
            <Section key={key} id={key} title={t(`project.sections.${key}`)}>
              {key === 'team' ? (
                <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  {project.team.map((member, index) => (
                    <li
                      key={`${member.name}-${String(index)}`}
                      className="flex items-center gap-3 rounded-xl border border-border bg-surface p-3"
                    >
                      <Avatar name={member.name} src={member.avatar_url} size="md" />
                      <span className="flex min-w-0 flex-col">
                        <span className="font-medium text-fg">{member.name}</span>
                        {member.role && (
                          <span className="text-sm text-fg-muted">
                            {TEAM_ROLES.has(member.role)
                              ? t(`teamRoles.${member.role as 'owner'}`)
                              : member.role}
                          </span>
                        )}
                      </span>
                    </li>
                  ))}
                </ul>
              ) : key === 'ask' ? (
                <div className="flex flex-col gap-5">
                  {card.needs.length > 0 && (
                    <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      {card.needs.map((need) => {
                        const Icon = NEED_ICONS[need];
                        return (
                          <li
                            key={need}
                            className="flex items-center gap-3 rounded-xl border border-gold-border bg-gold-soft p-3 text-gold-soft-fg"
                          >
                            <Icon className="size-5 shrink-0" aria-hidden />
                            <span className="font-medium">{t(`needsText.${need}`)}</span>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                  <Prose text={sections.ask} />
                </div>
              ) : (
                <Prose text={sections[key]} />
              )}
            </Section>
          ))}

          {card.tags.length > 0 && (
            <ul className="flex flex-wrap gap-2">
              {card.tags.map((tag) => (
                <li key={tag}>
                  <Link
                    to="/showcase"
                    search={{ tag }}
                    className="inline-flex h-8 items-center rounded-full border border-border bg-surface px-3 text-sm font-medium text-fg-muted transition-colors hover:border-border-strong hover:text-fg"
                  >
                    {tag}
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>

        <aside className="flex flex-col gap-5 lg:sticky lg:top-36 lg:self-start">
          {card.score !== null && card.level !== null && (
            <div className="flex flex-col gap-3 rounded-2xl border border-border bg-surface p-5 shadow-sm">
              <p className="text-sm font-semibold text-fg">{t('project.score.title')}</p>
              <p className="flex items-baseline gap-2">
                <span className="font-display text-5xl font-semibold text-fg tabular-nums">
                  {card.score}
                </span>
                <span className="text-sm text-fg-subtle">{t('project.score.outOf')}</span>
              </p>
              <Progress
                value={card.score}
                tone="brand"
                label={t('card.score', { score: card.score, level: t(`levels.${card.level}`) })}
              />
              <p className="text-sm font-medium text-fg">{t(`levels.${card.level}`)}</p>
              <p className="text-xs text-fg-subtle">{t('project.score.text')}</p>
            </div>
          )}
          <div className="flex flex-col gap-4 rounded-2xl border border-border bg-surface p-5 shadow-sm">
            <h2 className="text-sm font-semibold text-fg">{t('project.facts.label')}</h2>
            <dl className="grid grid-cols-1 gap-3 text-sm">
              <Fact label={t('project.facts.university')} value={text(card.university.name)} />
              {faculty && <Fact label={t('project.facts.faculty')} value={faculty} />}
              <Fact label={t('project.facts.industry')} value={t(`industries.${card.industry}`)} />
              <Fact label={t('project.facts.stage')} value={t(`stages.${card.stage}`)} />
              <Fact label={t('project.facts.year')} value={String(card.year)} />
              <Fact
                label={t('project.facts.published')}
                value={dateTime(card.published_at, { dateStyle: 'long' })}
              />
            </dl>
          </div>
          {project.contact_enabled && (
            <div className="flex flex-col gap-3 rounded-2xl border border-primary-border bg-primary-soft/50 p-5">
              <p className="text-sm leading-relaxed text-fg">{t('project.contactText')}</p>
              <Button
                leftIcon={<Mail />}
                onClick={() => {
                  setContactOpen(true);
                }}
              >
                {t('project.contact')}
              </Button>
            </div>
          )}
        </aside>
      </div>

      {project.contact_enabled && (
        <ContactDialog
          open={contactOpen}
          onOpenChange={setContactOpen}
          slug={card.slug}
          title={card.title}
        />
      )}
    </article>
  );
}

function Section({ id, title, children }: { id: SectionKey; title: string; children: ReactNode }) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="flex scroll-mt-32 flex-col gap-4">
      <h2 id={`${id}-title`} className="font-display text-2xl font-semibold text-fg sm:text-3xl">
        {title}
      </h2>
      {children}
    </section>
  );
}

/** Plain text of a section in paragraphs, at a readable measure. */
function Prose({ text }: { text: string | undefined }) {
  const paragraphs = (text ?? '')
    .split(/\n{2,}/)
    .map((p) => p.trim())
    .filter(Boolean);
  if (paragraphs.length === 0) return null;
  return (
    <div className="flex max-w-prose flex-col gap-4 text-lg leading-relaxed text-fg-muted">
      {paragraphs.map((p, i) => (
        <p key={i} className="whitespace-pre-line">
          {p}
        </p>
      ))}
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-fg-subtle">{label}</dt>
      <dd className="font-medium text-fg">{value}</dd>
    </div>
  );
}

function ShareMenu({ url, title }: { url: string; title: string }) {
  const { t } = useTranslation('showcase');
  const links = shareLinks(url, t('project.share.text', { title }));
  const open = (href: string) => {
    window.open(href, '_blank', 'noopener,noreferrer');
  };
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button size="lg" variant="secondary" leftIcon={<Share2 />}>
          {t('project.share.label')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56">
        <DropdownMenuItem
          icon={<Copy />}
          onSelect={() => {
            void navigator.clipboard.writeText(url).then(
              () => toast.success(t('project.share.copied')),
              () => toast.error(url),
            );
          }}
        >
          {t('project.share.copy')}
        </DropdownMenuItem>
        <DropdownMenuItem
          icon={<Send />}
          onSelect={() => {
            open(links.telegram);
          }}
        >
          {t('project.share.telegram')}
        </DropdownMenuItem>
        <DropdownMenuItem
          icon={<Share2 />}
          onSelect={() => {
            open(links.whatsapp);
          }}
        >
          {t('project.share.whatsapp')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The section being read: the last one whose heading passed under the sticky bars. */
function useActiveSection(keys: SectionKey[]): SectionKey | undefined {
  const [active, setActive] = useState<SectionKey | undefined>(keys[0]);
  const joined = keys.join(',');
  useEffect(() => {
    const ids = joined.split(',').filter(Boolean) as SectionKey[];
    const elements = ids
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null);
    if (elements.length === 0) return;
    const visible = new Map<string, boolean>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) visible.set(entry.target.id, entry.isIntersecting);
        const first = ids.find((id) => visible.get(id));
        if (first) setActive(first);
      },
      // The band under the header and the anchors, down to the middle of the screen.
      { rootMargin: '-128px 0px -50% 0px' },
    );
    for (const el of elements) observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [joined]);
  return active;
}

function ProjectSkeleton() {
  return (
    <div className="mx-auto flex max-w-page flex-col gap-6 px-4 pt-6 pb-16 sm:px-6" aria-busy>
      <Skeleton className="h-5 w-48" />
      <Skeleton className="aspect-[4/5] rounded-3xl sm:aspect-video lg:aspect-[21/9]" />
      <div className="grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-8 w-40" />
          <SkeletonText lines={5} />
        </div>
        <Skeleton className="h-64 rounded-2xl" />
      </div>
    </div>
  );
}

function ProjectError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation(['showcase', 'common']);
  const session = useQuery(sessionQuery);
  const missing = error instanceof ApiError && error.status === 404;
  return (
    <div className="mx-auto max-w-page px-4 py-16 sm:px-6">
      {missing ? (
        <EmptyState
          illustration="search"
          title={t('project.notFound.title')}
          description={t('project.notFound.text')}
          action={
            <Button asChild>
              <Link to="/showcase">{t('project.notFound.back')}</Link>
            </Button>
          }
          secondaryAction={
            session.data === null ? (
              <Button asChild variant="secondary">
                <Link to="/login" search={{ redirect: window.location.pathname }}>
                  {t('project.notFound.signIn')}
                </Link>
              </Button>
            ) : undefined
          }
        />
      ) : (
        <Callout
          tone="danger"
          title={t('project.error')}
          action={
            <Button size="sm" variant="secondary" onClick={onRetry}>
              {t('common:actions.retry')}
            </Button>
          }
        >
          {errorMessage(error)}
        </Callout>
      )}
    </div>
  );
}
