import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import {
  ArrowRight,
  Bot,
  Building2,
  ChartPie,
  Check,
  FlaskConical,
  Gauge,
  GraduationCap,
  Handshake,
  LayoutGrid,
  Lightbulb,
  Presentation,
  Search,
  ShieldCheck,
  Trophy,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { AnimatedNumber, Button, Card, Input, Kicker, LogoMark, Skeleton } from '@/design-system';
import { sessionQuery, tenantQuery } from '@/features/auth/api';
import { showcaseHighlightsQuery } from '@/features/showcase/api';
import { ShowcaseCardView } from '@/features/showcase/card';
import { SiteFrame, siteNavClass } from '@/features/showcase/site';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

const STAGES = [
  { key: 'idea', icon: Lightbulb, steps: ['s1', 's2', 's3', 's4'] },
  { key: 'market', icon: ChartPie, steps: ['s5', 's6', 's7', 's8'] },
  { key: 'product', icon: FlaskConical, steps: ['s9', 's10', 's11', 's12'] },
  { key: 'launch', icon: Presentation, steps: ['s13', 's14'] },
] as const;

const STEP_NUMBERS: Record<(typeof STAGES)[number]['steps'][number], number> = {
  s1: 1,
  s2: 2,
  s3: 3,
  s4: 4,
  s5: 5,
  s6: 6,
  s7: 7,
  s8: 8,
  s9: 9,
  s10: 10,
  s11: 11,
  s12: 12,
  s13: 13,
  s14: 14,
};

const HIGHLIGHTS: { key: 'mentor' | 'evaluation' | 'showcase'; icon: LucideIcon }[] = [
  { key: 'mentor', icon: Bot },
  { key: 'evaluation', icon: Gauge },
  { key: 'showcase', icon: Trophy },
];

const AUDIENCES: { key: 'universities' | 'partners'; icon: LucideIcon }[] = [
  { key: 'universities', icon: Building2 },
  { key: 'partners', icon: Handshake },
];

const POINTS = ['p1', 'p2', 'p3'] as const;

const NAV = ['how', 'students', 'universities', 'partners'] as const;

const COUNTERS: {
  key: 'projects' | 'published' | 'students' | 'evaluations';
  icon: LucideIcon;
}[] = [
  { key: 'projects', icon: Lightbulb },
  { key: 'published', icon: LayoutGrid },
  { key: 'students', icon: Users },
  { key: 'evaluations', icon: Gauge },
];

/** Public home page: what Qadam is, how the incubator works, who it is for. */
export function LandingPage() {
  const { t } = useTranslation('landing');
  const session = useQuery(sessionQuery);
  const signedIn = Boolean(session.data);

  return (
    <SiteFrame
      nav={NAV.map((key) => (
        <a key={key} href={`#${key}`} className={siteNavClass}>
          {t(`nav.${key}`)}
        </a>
      ))}
    >
      <Hero signedIn={signedIn} />
      <Counters />
      <HowItWorks />
      <Featured />
      <Audiences signedIn={signedIn} />
      <FinalCta signedIn={signedIn} />
    </SiteFrame>
  );
}

function Hero({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  return (
    <section className="relative overflow-hidden panel-ink" aria-labelledby="hero-title">
      <div aria-hidden className="absolute inset-0 ornament opacity-[0.025] [--qd-brand:#fff]" />
      <div
        aria-hidden
        className="absolute -top-56 -right-48 size-[44rem] rounded-full bg-primary-500/20 blur-3xl"
      />
      <div className="relative mx-auto grid max-w-page grid-cols-1 items-center gap-12 px-4 pt-14 pb-32 sm:px-6 lg:grid-cols-[minmax(0,1fr)_24rem] lg:pt-24 lg:pb-40">
        <div className="flex min-w-0 flex-col items-start gap-7">
          <Kicker tone="light">{t('hero.eyebrow')}</Kicker>
          <h1
            id="hero-title"
            className="font-display text-5xl font-extrabold text-white sm:text-6xl lg:text-7xl"
          >
            {t('hero.titleLead')} <em className="text-primary-300">{t('hero.titleAccent')}</em>
          </h1>
          <p className="max-w-2xl text-lg leading-relaxed text-white/80">{t('hero.subtitle')}</p>
          <HeroSearch />
          <Link
            to="/showcase"
            className="inline-flex items-center gap-2 border-b border-white/50 pb-1 text-sm font-bold text-white transition-colors hover:border-primary-300 hover:text-primary-300"
          >
            {t('hero.catalog')}
            <ArrowRight className="size-4" aria-hidden />
          </Link>
        </div>
        <UniversityCard signedIn={signedIn} />
      </div>
    </section>
  );
}

/** Search over the showcase: opens the catalogue with the query. */
function HeroSearch() {
  const { t } = useTranslation('landing');
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  return (
    <form
      role="search"
      aria-label={t('hero.searchLabel')}
      className="flex w-full max-w-2xl flex-col sm:flex-row"
      onSubmit={(event) => {
        event.preventDefault();
        const q = query.trim();
        void navigate({ to: '/showcase', search: q ? { q } : {} });
      }}
    >
      <div className="min-w-0 flex-1">
        <Input
          type="search"
          inputSize="lg"
          value={query}
          maxLength={100}
          onChange={(event) => {
            setQuery(event.target.value);
          }}
          aria-label={t('hero.searchLabel')}
          placeholder={t('hero.searchPlaceholder')}
          leading={<Search />}
          className="h-14 border-white/25 bg-white/5 text-base text-white placeholder:text-white/60 hover:border-white/45 focus-visible:border-primary-300"
        />
      </div>
      <Button type="submit" size="lg" className="h-14 px-7">
        {t('hero.search')}
      </Button>
    </form>
  );
}

/** The university the platform belongs to, what it offers, and the way in. */
function UniversityCard({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  return (
    <div className="flex flex-col gap-6 rounded-card border-t-2 border-primary-500 bg-deep-900/85 p-7 shadow-lg ring-1 ring-white/10 sm:p-8">
      {tenant.data?.logo_url ? (
        <img
          src={tenant.data.logo_url}
          alt=""
          className="size-20 rounded-full bg-white object-contain p-1"
        />
      ) : (
        <LogoMark className="size-14" />
      )}
      <div className="flex flex-col gap-3">
        {tenant.data ? (
          <p className="font-display text-2xl leading-snug font-extrabold text-white">
            {text(tenant.data.name)}
          </p>
        ) : (
          <Skeleton className="h-16 w-full opacity-20" />
        )}
        <p className="text-2xs font-bold tracking-kicker text-primary-300 uppercase">
          {t('hero.card.kicker')}
        </p>
      </div>
      <ul className="flex flex-col gap-3 border-t border-white/15 pt-6">
        {(['f1', 'f2', 'f3'] as const).map((feature) => (
          <li key={feature} className="flex gap-3 text-sm leading-relaxed text-white/85">
            <Check className="mt-0.5 size-4 shrink-0 text-primary-300" aria-hidden />
            {t(`hero.card.${feature}`)}
          </li>
        ))}
      </ul>
      <div className="flex flex-col gap-3">
        <Button asChild size="lg" block>
          <Link to={signedIn ? '/app' : '/register'}>
            {signedIn ? t('actions.toApp') : t('hero.primary')}
            <ArrowRight aria-hidden />
          </Link>
        </Button>
        {!signedIn && (
          <p className="flex items-center gap-2 text-xs text-white/70">
            <ShieldCheck className="size-4 shrink-0 text-primary-300" aria-hidden />
            {t('hero.note')}
          </p>
        )}
      </div>
    </div>
  );
}

function SectionHeading({
  id,
  eyebrow,
  title,
  text,
}: {
  id: string;
  eyebrow: string;
  title: string;
  text: string;
}) {
  return (
    <div className="flex max-w-3xl flex-col gap-4">
      <Kicker>{eyebrow}</Kicker>
      <h2 id={id} className="font-display text-4xl font-extrabold text-ink sm:text-5xl">
        {title}
      </h2>
      <p className="text-lg leading-relaxed text-fg-muted">{text}</p>
    </div>
  );
}

/** Live numbers of the university (B-012), laid over the hero's edge; hidden without the API. */
function Counters() {
  const { t } = useTranslation('landing');
  const { number } = useFormatters();
  const highlights = useQuery(showcaseHighlightsQuery);
  if (highlights.isError) return null;
  const counters = highlights.data?.counters;
  return (
    <section
      aria-label={t('counters.label')}
      className="relative z-10 mx-auto -mt-20 max-w-page px-4 sm:px-6"
    >
      <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-card border border-border bg-border shadow-lg lg:grid-cols-4">
        {COUNTERS.map(({ key, icon: Icon }) => (
          <div key={key} className="flex flex-col-reverse gap-2 bg-surface p-5 sm:p-7">
            <dt className="text-sm text-fg-muted">{t(`counters.${key}`)}</dt>
            <dd className="flex items-center gap-3 font-figures text-3xl font-bold text-ink sm:text-5xl">
              <Icon className="size-5 shrink-0 text-primary-text" aria-hidden />
              {counters ? (
                <AnimatedNumber
                  value={counters[key]}
                  format={(value) => number(Math.round(value))}
                />
              ) : (
                <Skeleton className="h-9 w-20 sm:h-12" />
              )}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function HowItWorks() {
  const { t } = useTranslation('landing');
  return (
    <section id="how" aria-labelledby="how-title" className="scroll-mt-20">
      <div className="mx-auto flex max-w-page flex-col gap-12 px-4 py-20 sm:px-6 lg:py-28">
        <SectionHeading
          id="how-title"
          eyebrow={t('how.eyebrow')}
          title={t('how.title')}
          text={t('how.subtitle')}
        />
        <ol className="relative grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          <span
            aria-hidden
            className="absolute inset-x-12 top-12 hidden h-px bg-primary-border lg:block"
          />
          {STAGES.map(({ key, icon: Icon, steps }, index) => (
            <li key={key} className="relative">
              <Card className="h-full gap-5 p-6">
                <div className="flex items-center justify-between">
                  <span className="grid size-12 place-items-center rounded-full border border-primary-border bg-surface text-primary-text">
                    <Icon className="size-5" aria-hidden />
                  </span>
                  <span className="text-2xs font-bold tracking-label text-primary-text uppercase">
                    {t('how.stageNumber', { n: index + 1 })}
                  </span>
                </div>
                <div className="flex flex-col gap-2">
                  <h3 className="text-xl font-bold text-ink">{t(`how.stages.${key}.title`)}</h3>
                  <p className="text-sm leading-relaxed text-fg-muted">
                    {t(`how.stages.${key}.text`)}
                  </p>
                </div>
                <ul className="mt-auto flex flex-col gap-2 border-t border-border pt-4">
                  {steps.map((step) => (
                    <li key={step} className="flex gap-2.5 text-sm text-fg">
                      <span className="mt-px grid size-5 shrink-0 place-items-center rounded-full bg-primary-soft text-2xs font-bold text-primary-soft-fg tabular-nums">
                        {STEP_NUMBERS[step]}
                      </span>
                      {t(`how.steps.${step}`)}
                    </li>
                  ))}
                </ul>
              </Card>
            </li>
          ))}
        </ol>
        <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
          {HIGHLIGHTS.map(({ key, icon: Icon }) => (
            <Card key={key} className="gap-4 p-6">
              <span className="grid size-12 place-items-center rounded-lg bg-primary-soft text-primary-text">
                <Icon className="size-5" aria-hidden />
              </span>
              <div className="flex flex-col gap-2">
                <h3 className="text-xl font-bold text-ink">{t(`how.highlights.${key}.title`)}</h3>
                <p className="text-sm leading-relaxed text-fg-muted">
                  {t(`how.highlights.${key}.text`)}
                </p>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
}

/** The best published projects of the showcase; the block is hidden until there are some. */
function Featured() {
  const { t } = useTranslation('landing');
  const highlights = useQuery(showcaseHighlightsQuery);
  const tenant = useQuery(tenantQuery);
  const featured = highlights.data?.featured ?? [];
  if (featured.length === 0) return null;
  return (
    <section aria-labelledby="featured-title" className="border-y border-border bg-surface">
      <div className="mx-auto flex max-w-page flex-col gap-10 px-4 py-20 sm:px-6 lg:py-28">
        <div className="flex flex-wrap items-end justify-between gap-6">
          <SectionHeading
            id="featured-title"
            eyebrow={t('featured.eyebrow')}
            title={t('featured.title')}
            text={t('featured.text')}
          />
          <Button asChild variant="link">
            <Link to="/showcase">
              {t('featured.all')}
              <ArrowRight aria-hidden />
            </Link>
          </Button>
        </div>
        <ul className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {featured.slice(0, 6).map((card) => (
            <li key={`${card.university.slug}/${card.slug}`}>
              <ShowcaseCardView
                card={card}
                external={tenant.data !== undefined && card.university.slug !== tenant.data.slug}
                showUniversity={false}
              />
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

function Audiences({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  return (
    <section
      aria-labelledby="audiences-title"
      className="mx-auto max-w-page px-4 py-20 sm:px-6 lg:py-28"
    >
      <div className="flex flex-col gap-12">
        <SectionHeading
          id="audiences-title"
          eyebrow={t('audiences.eyebrow')}
          title={t('audiences.title')}
          text={t('audiences.subtitle')}
        />
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <section
            id="students"
            aria-labelledby="students-title"
            className="flex scroll-mt-24 flex-col gap-6 rounded-card border-t-2 border-primary-500 panel-deep p-7 sm:p-9 lg:row-span-2"
          >
            <span className="grid size-12 place-items-center rounded-lg bg-white/10 text-primary-300">
              <GraduationCap className="size-5" aria-hidden />
            </span>
            <div className="flex flex-col gap-3">
              <Kicker tone="light">{t('audiences.students.eyebrow')}</Kicker>
              <h3
                id="students-title"
                className="font-display text-3xl font-extrabold text-white sm:text-4xl"
              >
                {t('audiences.students.title')}
              </h3>
            </div>
            <ul className="flex flex-col gap-3 border-t border-white/15 pt-6">
              {POINTS.map((point) => (
                <li key={point} className="flex gap-3 leading-relaxed text-white/85">
                  <Check className="mt-1 size-4 shrink-0 text-primary-300" aria-hidden />
                  {t(`audiences.students.${point}`)}
                </li>
              ))}
            </ul>
            <div className="mt-auto pt-2">
              <Button asChild size="lg">
                <Link to={signedIn ? '/app' : '/register'}>
                  {signedIn ? t('actions.toApp') : t('audiences.students.cta')}
                  <ArrowRight aria-hidden />
                </Link>
              </Button>
            </div>
          </section>
          {AUDIENCES.map(({ key, icon: Icon }) => (
            <Card
              key={key}
              id={key}
              aria-labelledby={`${key}-title`}
              className="scroll-mt-24 gap-5 p-7"
            >
              <div className="flex items-start gap-4">
                <span className="grid size-12 shrink-0 place-items-center rounded-lg bg-primary-soft text-primary-text">
                  <Icon className="size-5" aria-hidden />
                </span>
                <div className="flex flex-col gap-1.5">
                  <p className="text-2xs font-bold tracking-label text-primary-text uppercase">
                    {t(`audiences.${key}.eyebrow`)}
                  </p>
                  <h3 id={`${key}-title`} className="text-2xl font-bold text-ink">
                    {t(`audiences.${key}.title`)}
                  </h3>
                </div>
              </div>
              <ul className="flex flex-col gap-2.5">
                {POINTS.map((point) => (
                  <li key={point} className="flex gap-2.5 text-sm leading-relaxed text-fg-muted">
                    <Check className="mt-0.5 size-4 shrink-0 text-primary-text" aria-hidden />
                    {t(`audiences.${key}.${point}`)}
                  </li>
                ))}
              </ul>
              <div className="mt-auto pt-1">
                <Button asChild variant="link">
                  {key === 'universities' ? (
                    <Link to={signedIn ? '/app' : '/login'}>
                      {t('audiences.universities.cta')}
                      <ArrowRight aria-hidden />
                    </Link>
                  ) : (
                    <Link to="/showcase">
                      {t('audiences.partners.cta')}
                      <ArrowRight aria-hidden />
                    </Link>
                  )}
                </Button>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </section>
  );
}

function FinalCta({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  return (
    <section className="mx-auto max-w-page px-4 pb-20 sm:px-6 lg:pb-28" aria-labelledby="cta-title">
      <div className="relative overflow-hidden rounded-card panel-ink px-6 py-12 sm:px-12 lg:py-14">
        <div aria-hidden className="absolute inset-0 ornament opacity-[0.05] [--qd-brand:#fff]" />
        <div className="relative grid grid-cols-1 items-center gap-8 lg:grid-cols-[minmax(0,1fr)_auto]">
          <div className="flex flex-col gap-4">
            <Kicker tone="light">{t('cta.kicker')}</Kicker>
            <h2
              id="cta-title"
              className="font-display text-3xl font-extrabold text-white sm:text-5xl"
            >
              {t('cta.title')}
            </h2>
            <p className="max-w-xl text-lg text-white/80">{t('cta.text')}</p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row lg:flex-col">
            <Button asChild size="lg">
              <Link to={signedIn ? '/app' : '/register'}>
                {signedIn ? t('actions.toApp') : t('cta.primary')}
                <ArrowRight aria-hidden />
              </Link>
            </Button>
            {!signedIn && (
              <Button asChild size="lg" variant="light">
                <Link to="/login">{t('actions.signIn')}</Link>
              </Button>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
