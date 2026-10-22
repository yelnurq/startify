import { useQuery } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
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
  Lightbulb,
  Presentation,
  ShieldCheck,
  Sparkles,
  Trophy,
  type LucideIcon,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { AnimatedNumber, Badge, Button, Card, cn, Skeleton } from '@/design-system';
import { sessionQuery, tenantQuery } from '@/features/auth/api';
import { showcaseHighlightsQuery } from '@/features/showcase/api';
import { ShowcaseCardView } from '@/features/showcase/card';
import { SiteFrame, siteNavClass } from '@/features/showcase/site';
import { useFormatters } from '@/shared/i18n/format';

import { RouteTimeline, RouteVisual } from './route-visual';

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

const AUDIENCES: { key: 'students' | 'universities' | 'partners'; icon: LucideIcon }[] = [
  { key: 'students', icon: GraduationCap },
  { key: 'universities', icon: Building2 },
  { key: 'partners', icon: Handshake },
];

const NAV = ['how', 'students', 'universities', 'partners'] as const;

const COUNTERS = ['projects', 'published', 'students', 'evaluations'] as const;

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
    <section className="relative overflow-hidden" aria-labelledby="hero-title">
      <div aria-hidden className="absolute inset-0 ornament-fade opacity-[0.06]" />
      <div
        aria-hidden
        className="absolute -top-56 left-1/2 h-[30rem] w-[60rem] -translate-x-1/2 rounded-full bg-primary-300/30 blur-3xl dark:bg-primary-600/15"
      />
      <div className="relative mx-auto grid max-w-page grid-cols-1 items-center gap-12 px-4 py-14 sm:px-6 lg:grid-cols-[1.05fr_1fr] lg:py-24">
        <div className="flex flex-col items-start gap-6">
          <Badge tone="primary" size="lg" icon={<Sparkles />}>
            {t('hero.eyebrow')}
          </Badge>
          <h1
            id="hero-title"
            className="font-display text-5xl font-semibold text-balance text-fg sm:text-6xl"
          >
            {t('hero.title')}
          </h1>
          <p className="max-w-xl text-lg leading-relaxed text-fg-muted">{t('hero.subtitle')}</p>
          <div className="flex flex-wrap gap-3">
            <Button asChild size="lg">
              <Link to={signedIn ? '/app' : '/register'}>
                {signedIn ? t('actions.toApp') : t('hero.primary')}
                <ArrowRight aria-hidden />
              </Link>
            </Button>
            <Button asChild size="lg" variant="secondary">
              <Link to="/showcase">{t('hero.secondary')}</Link>
            </Button>
          </div>
          <p className="flex items-center gap-2 text-sm text-fg-subtle">
            <ShieldCheck className="size-4 text-success" aria-hidden />
            {t('hero.note')}
          </p>
        </div>
        <RouteVisual className="hidden lg:block" />
        <RouteTimeline className="lg:hidden" />
      </div>
    </section>
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
    <div className="flex max-w-2xl flex-col gap-3">
      <p className="text-sm font-semibold text-primary-text">{eyebrow}</p>
      <h2 id={id} className="font-display text-3xl font-semibold text-balance text-fg sm:text-4xl">
        {title}
      </h2>
      <p className="text-lg leading-relaxed text-fg-muted">{text}</p>
    </div>
  );
}

function HowItWorks() {
  const { t } = useTranslation('landing');
  return (
    <section
      id="how"
      aria-labelledby="how-title"
      className="scroll-mt-16 border-y border-border bg-surface"
    >
      <div className="mx-auto flex max-w-page flex-col gap-12 px-4 py-16 sm:px-6 lg:py-24">
        <SectionHeading
          id="how-title"
          eyebrow={t('how.eyebrow')}
          title={t('how.title')}
          text={t('how.subtitle')}
        />
        <ol className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {STAGES.map(({ key, icon: Icon, steps }, index) => (
            <li key={key}>
              <Card className="h-full gap-5 p-6">
                <div className="flex items-center justify-between">
                  <span className="grid size-11 place-items-center rounded-xl bg-primary-soft text-primary-soft-fg">
                    <Icon className="size-5" aria-hidden />
                  </span>
                  <span className="font-display text-sm font-semibold text-fg-subtle tabular-nums">
                    {t('how.stageNumber', { n: index + 1 })}
                  </span>
                </div>
                <div className="flex flex-col gap-2">
                  <h3 className="text-lg font-semibold text-fg">{t(`how.stages.${key}.title`)}</h3>
                  <p className="text-sm leading-relaxed text-fg-muted">
                    {t(`how.stages.${key}.text`)}
                  </p>
                </div>
                <ul className="mt-auto flex flex-col gap-2 border-t border-border pt-4">
                  {steps.map((step) => (
                    <li key={step} className="flex gap-2.5 text-sm text-fg">
                      <span className="mt-px grid size-5 shrink-0 place-items-center rounded-full bg-surface-sunken text-[11px] font-semibold text-fg-muted tabular-nums">
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
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {HIGHLIGHTS.map(({ key, icon: Icon }) => (
            <div key={key} className="flex gap-3 rounded-xl bg-surface-raised p-4">
              <Icon className="mt-0.5 size-5 shrink-0 text-primary-text" aria-hidden />
              <div className="flex flex-col gap-1">
                <p className="font-semibold text-fg">{t(`how.highlights.${key}.title`)}</p>
                <p className="text-sm text-fg-muted">{t(`how.highlights.${key}.text`)}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function Audiences({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  return (
    <section
      aria-labelledby="audiences-title"
      className="mx-auto max-w-page px-4 py-16 sm:px-6 lg:py-24"
    >
      <div className="flex flex-col gap-12">
        <SectionHeading
          id="audiences-title"
          eyebrow={t('audiences.eyebrow')}
          title={t('audiences.title')}
          text={t('audiences.subtitle')}
        />
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
          {AUDIENCES.map(({ key, icon: Icon }) => (
            <Card
              key={key}
              id={key}
              aria-labelledby={`${key}-title`}
              className={cn(
                'scroll-mt-20 gap-5 p-6',
                key === 'students' && 'border-primary-border bg-primary-soft/40',
              )}
            >
              <span className="grid size-11 place-items-center rounded-xl bg-surface text-primary-text shadow-sm">
                <Icon className="size-5" aria-hidden />
              </span>
              <div className="flex flex-col gap-2">
                <p className="text-sm font-semibold text-primary-text">
                  {t(`audiences.${key}.eyebrow`)}
                </p>
                <h3 id={`${key}-title`} className="font-display text-xl font-semibold text-fg">
                  {t(`audiences.${key}.title`)}
                </h3>
              </div>
              <ul className="flex flex-col gap-2.5">
                {(['p1', 'p2', 'p3'] as const).map((point) => (
                  <li key={point} className="flex gap-2.5 text-sm leading-relaxed text-fg-muted">
                    <Check className="mt-0.5 size-4 shrink-0 text-success" aria-hidden />
                    {t(`audiences.${key}.${point}`)}
                  </li>
                ))}
              </ul>
              <div className="mt-auto pt-2">
                {key === 'students' && (
                  <Button asChild>
                    <Link to={signedIn ? '/app' : '/register'}>
                      {signedIn ? t('actions.toApp') : t('audiences.students.cta')}
                      <ArrowRight aria-hidden />
                    </Link>
                  </Button>
                )}
                {key === 'universities' && (
                  <Button asChild variant="secondary">
                    <Link to={signedIn ? '/app' : '/login'}>{t('audiences.universities.cta')}</Link>
                  </Button>
                )}
                {key === 'partners' && (
                  <Button asChild variant="secondary">
                    <Link to="/showcase">
                      {t('audiences.partners.cta')}
                      <ArrowRight aria-hidden />
                    </Link>
                  </Button>
                )}
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
    <section className="mx-auto max-w-page px-4 pb-16 sm:px-6 lg:pb-24" aria-labelledby="cta-title">
      <div className="relative overflow-hidden rounded-3xl bg-linear-to-br from-primary-600 via-primary-800 to-primary-950 px-6 py-12 text-center shadow-lg sm:px-12 sm:py-16">
        <div aria-hidden className="absolute inset-0 ornament opacity-[0.08]" />
        <div className="relative mx-auto flex max-w-2xl flex-col items-center gap-5">
          <h2
            id="cta-title"
            className="font-display text-3xl font-semibold text-balance text-white sm:text-4xl"
          >
            {t('cta.title')}
          </h2>
          <p className="text-lg text-primary-100">{t('cta.text')}</p>
          <div className="flex flex-wrap justify-center gap-3">
            <Button asChild size="lg" variant="gold">
              <Link to={signedIn ? '/app' : '/register'}>
                {signedIn ? t('actions.toApp') : t('cta.primary')}
                <ArrowRight aria-hidden />
              </Link>
            </Button>
            {!signedIn && (
              <Button
                asChild
                size="lg"
                variant="ghost"
                className="text-white hover:bg-white/10 hover:text-white"
              >
                <Link to="/login">{t('actions.signIn')}</Link>
              </Button>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}

/** Live numbers of the university (B-012); hidden when the API is unavailable. */
function Counters() {
  const { t } = useTranslation('landing');
  const { number } = useFormatters();
  const highlights = useQuery(showcaseHighlightsQuery);
  if (highlights.isError) return null;
  const counters = highlights.data?.counters;
  return (
    <section
      aria-label={t('counters.label')}
      className="mx-auto max-w-page px-4 pb-14 sm:px-6 lg:pb-20"
    >
      <dl className="grid grid-cols-2 gap-x-4 gap-y-6 rounded-2xl border border-border bg-surface p-5 shadow-sm sm:p-6 lg:grid-cols-4">
        {COUNTERS.map((key) => (
          <div key={key} className="flex flex-col-reverse gap-1">
            <dt className="text-sm text-fg-muted">{t(`counters.${key}`)}</dt>
            <dd className="font-display text-3xl font-semibold text-fg tabular-nums sm:text-4xl">
              {counters ? (
                <AnimatedNumber
                  value={counters[key]}
                  format={(value) => number(Math.round(value))}
                />
              ) : (
                <Skeleton className="h-9 w-20 sm:h-10" />
              )}
            </dd>
          </div>
        ))}
      </dl>
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
    <section
      aria-labelledby="featured-title"
      className="mx-auto max-w-page px-4 pt-16 sm:px-6 lg:pt-24"
    >
      <div className="flex flex-col gap-10">
        <div className="flex flex-wrap items-end justify-between gap-6">
          <SectionHeading
            id="featured-title"
            eyebrow={t('featured.eyebrow')}
            title={t('featured.title')}
            text={t('featured.text')}
          />
          <Button asChild variant="secondary">
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
