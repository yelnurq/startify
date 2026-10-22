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
  Lightbulb,
  Presentation,
  Search,
  Trophy,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import {
  AnimatedNumber,
  BrandMark,
  Button,
  EdgeIllustration,
  IdeaIllustration,
  Illustration,
  Input,
  Kicker,
  Skeleton,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/design-system';
import { sessionQuery, tenantQuery } from '@/features/auth/api';
import { showcaseHighlightsQuery } from '@/features/showcase/api';
import { ShowcaseCardView } from '@/features/showcase/card';
import { SiteFrame, siteNavClass } from '@/features/showcase/site';
import { useFormatters } from '@/shared/i18n/format';
import { useLocalized } from '@/shared/i18n/localized';

const STAGES = [
  { key: 'idea', icon: Lightbulb, art: 'idea', steps: ['s1', 's2', 's3', 's4'] },
  { key: 'market', icon: ChartPie, art: 'target', steps: ['s5', 's6', 's7', 's8'] },
  { key: 'product', icon: FlaskConical, art: 'lab', steps: ['s9', 's10', 's11', 's12'] },
  { key: 'launch', icon: Presentation, art: 'pitch', steps: ['s13', 's14'] },
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

const ROUTE = ['idea', 'incubator', 'evaluation', 'showcase', 'demoDay', 'market'] as const;

const AUDIENCES = ['students', 'universities', 'partners'] as const;
type Audience = (typeof AUDIENCES)[number];

const AUDIENCE_ICONS: Record<Audience, LucideIcon> = {
  students: GraduationCap,
  universities: Building2,
  partners: Handshake,
};

const POINTS = ['p1', 'p2', 'p3'] as const;

const NAV = ['how', 'students', 'universities', 'partners'] as const;

const COUNTERS = ['projects', 'published', 'students', 'evaluations'] as const;

function isAudience(value: string): value is Audience {
  return (AUDIENCES as readonly string[]).includes(value);
}

/** Public home page: what Qadam is, the route of a project, who it is for. */
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
      <Numbers />
      <HowItWorks />
      <Featured />
      <Audiences signedIn={signedIn} />
      <FinalCta signedIn={signedIn} />
    </SiteFrame>
  );
}

/** The university's photo with the promise of the platform and the route of a project. */
function Hero({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  const tenant = useQuery(tenantQuery);
  const cover = tenant.data?.cover_url;
  return (
    <section className="relative isolate overflow-hidden bg-deep-900" aria-labelledby="hero-title">
      {cover && (
        <img
          src={cover}
          alt=""
          width={1200}
          height={461}
          fetchPriority="high"
          className="absolute inset-0 -z-10 size-full object-cover"
        />
      )}
      <div
        aria-hidden
        className="absolute inset-0 -z-10 bg-linear-to-r from-ink-800/95 via-deep-900/80 to-deep-900/30"
      />
      <div className="mx-auto flex max-w-page flex-col gap-12 px-4 pt-16 pb-8 sm:px-6 lg:pt-28 lg:pb-10">
        <div className="flex max-w-3xl flex-col items-start gap-6">
          <Kicker tone="light">{t('hero.eyebrow')}</Kicker>
          <h1
            id="hero-title"
            className="font-display text-5xl font-extrabold text-white sm:text-6xl lg:text-7xl"
          >
            {t('hero.titleLead')} <em className="text-primary-300">{t('hero.titleAccent')}</em>
          </h1>
          <p className="max-w-2xl text-lg leading-relaxed text-white/85">{t('hero.subtitle')}</p>
          <div className="flex flex-wrap gap-3">
            <Button asChild size="lg">
              <Link to={signedIn ? '/app' : '/register'}>
                {signedIn ? t('actions.toApp') : t('hero.primary')}
                <ArrowRight aria-hidden />
              </Link>
            </Button>
            <Button asChild size="lg" variant="light">
              <Link to="/showcase">{t('hero.secondary')}</Link>
            </Button>
          </div>
        </div>
        <ol
          aria-label={t('route.label')}
          className="grid grid-cols-2 gap-px overflow-hidden rounded-card border border-white/15 bg-white/15 sm:grid-cols-3 lg:grid-cols-6"
        >
          {ROUTE.map((key, index) => (
            <li key={key} className="flex flex-col gap-1 bg-ink-950/55 p-4 backdrop-blur-sm">
              <span className="font-figures text-xs font-bold tracking-label text-primary-300">
                {String(index + 1).padStart(2, '0')}
              </span>
              <span className="font-semibold text-white">{t(`route.${key}`)}</span>
              <span className="text-xs leading-snug text-white/75">{t(`route.hints.${key}`)}</span>
            </li>
          ))}
        </ol>
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
    <div className="flex max-w-3xl flex-col gap-4">
      <Kicker>{eyebrow}</Kicker>
      <h2 id={id} className="font-display text-4xl font-extrabold text-ink sm:text-5xl">
        {title}
      </h2>
      <p className="text-lg leading-relaxed text-fg-muted">{text}</p>
    </div>
  );
}

/** Live numbers of the university (B-012); hidden when the API is unavailable. */
function Numbers() {
  const { t } = useTranslation('landing');
  const { number } = useFormatters();
  const text = useLocalized();
  const highlights = useQuery(showcaseHighlightsQuery);
  const tenant = useQuery(tenantQuery);
  if (highlights.isError) return null;
  const counters = highlights.data?.counters;
  const university = tenant.data ? text(tenant.data.short_name) : '';
  return (
    <section aria-labelledby="numbers-title" className="border-b border-border bg-surface">
      <div className="mx-auto grid max-w-page grid-cols-1 gap-10 px-4 py-16 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:items-center lg:py-20">
        <div className="flex flex-col gap-3">
          <Kicker>{t('counters.kicker')}</Kicker>
          <h2
            id="numbers-title"
            className="font-display text-3xl font-extrabold text-ink sm:text-4xl"
          >
            {t('counters.title', { university })}
          </h2>
          <p className="leading-relaxed text-fg-muted">{t('counters.text')}</p>
          <Illustration name="growth" className="hidden max-w-48 lg:block" />
        </div>
        <dl className="grid grid-cols-2 gap-y-8 sm:grid-cols-4">
          {COUNTERS.map((key) => (
            <div
              key={key}
              className="flex flex-col-reverse gap-1 border-l-2 border-primary/30 pl-5"
            >
              <dt className="text-sm text-fg-muted">{t(`counters.${key}`)}</dt>
              <dd className="font-figures text-4xl font-bold text-ink sm:text-5xl">
                {counters ? (
                  <AnimatedNumber
                    value={counters[key]}
                    format={(value) => number(Math.round(value))}
                  />
                ) : (
                  <Skeleton className="h-10 w-20 sm:h-12" />
                )}
              </dd>
            </div>
          ))}
        </dl>
      </div>
    </section>
  );
}

function HowItWorks() {
  const { t } = useTranslation('landing');
  return (
    <section id="how" aria-labelledby="how-title" className="relative scroll-mt-20 overflow-x-clip">
      <EdgeIllustration
        name="route"
        placement="bottom-left"
        className="z-10 hidden w-72 translate-y-1/2 rotate-6 lg:block"
      />
      <div className="mx-auto grid max-w-page grid-cols-1 gap-12 px-4 py-20 sm:px-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-16 lg:py-28">
        <div className="flex flex-col gap-10 lg:sticky lg:top-28 lg:self-start">
          <SectionHeading
            id="how-title"
            eyebrow={t('how.eyebrow')}
            title={t('how.title')}
            text={t('how.subtitle')}
          />
          <ul className="flex flex-col gap-5 border-t border-border pt-8">
            {HIGHLIGHTS.map(({ key, icon: Icon }) => (
              <li key={key} className="flex gap-4">
                <span className="grid size-11 shrink-0 place-items-center rounded-lg bg-primary-soft text-primary-text">
                  <Icon className="size-5" aria-hidden />
                </span>
                <div className="flex flex-col gap-1">
                  <p className="font-semibold text-ink">{t(`how.highlights.${key}.title`)}</p>
                  <p className="text-sm leading-relaxed text-fg-muted">
                    {t(`how.highlights.${key}.text`)}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <ol className="flex flex-col">
          {STAGES.map(({ key, icon: Icon, art, steps }, index) => (
            <li
              key={key}
              className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-6 border-t border-border py-8 first:border-t-0 first:pt-0 xl:grid-cols-[auto_minmax(0,1fr)_9rem]"
            >
              <span
                aria-hidden
                className="font-display text-5xl leading-none font-extrabold text-primary tabular-nums"
              >
                {String(index + 1).padStart(2, '0')}
              </span>
              <div className="flex flex-col gap-3">
                <p className="flex items-center gap-2 text-2xs font-bold tracking-label text-primary-text uppercase">
                  <Icon className="size-4" aria-hidden />
                  {t('how.stageNumber', { n: index + 1 })}
                </p>
                <h3 className="text-2xl font-bold text-ink">{t(`how.stages.${key}.title`)}</h3>
                <p className="leading-relaxed text-fg-muted">{t(`how.stages.${key}.text`)}</p>
                <ul className="flex flex-wrap gap-2 pt-1">
                  {steps.map((step) => (
                    <li
                      key={step}
                      className="inline-flex items-center gap-2 rounded-full border border-border bg-surface py-1 pr-3 pl-1 text-sm text-fg"
                    >
                      <span className="grid size-6 place-items-center rounded-full bg-primary-soft text-2xs font-bold text-primary-soft-fg tabular-nums">
                        {STEP_NUMBERS[step]}
                      </span>
                      {t(`how.steps.${step}`)}
                    </li>
                  ))}
                </ul>
              </div>
              <Illustration name={art} className="hidden w-36 self-center xl:block" />
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

/** Search over the showcase: opens the catalogue with the query. */
function ShowcaseSearch() {
  const { t } = useTranslation('landing');
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  return (
    <form
      role="search"
      aria-label={t('hero.searchLabel')}
      className="flex flex-col gap-3"
      onSubmit={(event) => {
        event.preventDefault();
        const q = query.trim();
        void navigate({ to: '/showcase', search: q ? { q } : {} });
      }}
    >
      <p className="font-semibold text-ink">{t('featured.searchTitle')}</p>
      <div className="flex flex-col sm:flex-row lg:flex-col xl:flex-row">
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
            className="border-t-2 border-t-primary-500"
          />
        </div>
        <Button type="submit" size="lg" variant="ink">
          {t('hero.search')}
        </Button>
      </div>
    </form>
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
      className="relative overflow-x-clip border-y border-border bg-surface"
    >
      <EdgeIllustration
        name="pitch"
        placement="top-right"
        className="hidden w-64 -translate-y-1/2 -rotate-6 lg:block"
      />
      <div className="mx-auto grid max-w-page grid-cols-1 gap-10 px-4 py-20 sm:px-6 lg:grid-cols-[minmax(0,4fr)_minmax(0,8fr)] lg:gap-14 lg:py-28">
        <div className="flex flex-col gap-8 lg:sticky lg:top-28 lg:self-start">
          <SectionHeading
            id="featured-title"
            eyebrow={t('featured.eyebrow')}
            title={t('featured.title')}
            text={t('featured.text')}
          />
          <ShowcaseSearch />
          <Button asChild variant="link" className="self-start">
            <Link to="/showcase">
              {t('featured.all')}
              <ArrowRight aria-hidden />
            </Link>
          </Button>
        </div>
        <ul className="grid grid-cols-1 gap-5 sm:grid-cols-2">
          {featured.slice(0, 4).map((card) => (
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

/** The audience named in the address (#students and so on), as in the header links. */
function useAudienceFromHash(): [Audience, (value: Audience) => void] {
  const [audience, setAudience] = useState<Audience>('students');
  useEffect(() => {
    const sync = () => {
      const hash = window.location.hash.slice(1);
      if (isAudience(hash)) setAudience(hash);
    };
    sync();
    window.addEventListener('hashchange', sync);
    return () => {
      window.removeEventListener('hashchange', sync);
    };
  }, []);
  return [audience, setAudience];
}

const AUDIENCE_ART: Record<Audience, ReactNode> = {
  students: <IdeaIllustration className="w-full max-w-xs" />,
  universities: <Illustration name="university" className="w-full max-w-xs" />,
  partners: <Illustration name="partners" className="w-full max-w-xs" />,
};

function Audiences({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  const [audience, setAudience] = useAudienceFromHash();
  return (
    <section aria-labelledby="audiences-title" className="relative overflow-x-clip">
      <EdgeIllustration
        name="puzzle"
        placement="top-right"
        className="hidden w-64 translate-y-1/4 rotate-12 lg:block"
      />
      <div className="mx-auto flex max-w-page flex-col gap-10 px-4 py-20 sm:px-6 lg:py-28">
        <SectionHeading
          id="audiences-title"
          eyebrow={t('audiences.eyebrow')}
          title={t('audiences.title')}
          text={t('audiences.subtitle')}
        />
        {/* Targets of the header links (#students and so on); the tab follows the address. */}
        {AUDIENCES.map((key) => (
          <span key={key} id={key} aria-hidden className="-mt-10 block scroll-mt-28" />
        ))}
        <Tabs
          value={audience}
          onValueChange={(value) => {
            if (isAudience(value)) setAudience(value);
          }}
        >
          <TabsList aria-label={t('audiences.tabsLabel')}>
            {AUDIENCES.map((key) => {
              const Icon = AUDIENCE_ICONS[key];
              return (
                <TabsTrigger key={key} value={key} icon={<Icon />}>
                  {t(`audiences.${key}.eyebrow`)}
                </TabsTrigger>
              );
            })}
          </TabsList>
          {AUDIENCES.map((key) => (
            <TabsContent key={key} value={key} className="pt-8">
              <div className="grid grid-cols-1 items-center gap-10 rounded-card border border-border bg-surface p-6 sm:p-10 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
                <div className="flex flex-col gap-6">
                  <h3 className="font-display text-3xl font-extrabold text-ink sm:text-4xl">
                    {t(`audiences.${key}.title`)}
                  </h3>
                  <ul className="flex flex-col gap-3">
                    {POINTS.map((point) => (
                      <li key={point} className="flex gap-3 leading-relaxed text-fg">
                        <Check className="mt-1 size-4 shrink-0 text-primary-text" aria-hidden />
                        {t(`audiences.${key}.${point}`)}
                      </li>
                    ))}
                  </ul>
                  <div>
                    <AudienceAction audience={key} signedIn={signedIn} />
                  </div>
                </div>
                <div className="hidden justify-center rounded-card bg-primary-soft/60 p-8 sm:flex">
                  {AUDIENCE_ART[key]}
                </div>
              </div>
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </section>
  );
}

function AudienceAction({ audience, signedIn }: { audience: Audience; signedIn: boolean }) {
  const { t } = useTranslation('landing');
  if (audience === 'students') {
    return (
      <Button asChild size="lg">
        <Link to={signedIn ? '/app' : '/register'}>
          {signedIn ? t('actions.toApp') : t('audiences.students.cta')}
          <ArrowRight aria-hidden />
        </Link>
      </Button>
    );
  }
  if (audience === 'universities') {
    return (
      <Button asChild size="lg" variant="ink">
        <Link to={signedIn ? '/app' : '/login'}>{t('audiences.universities.cta')}</Link>
      </Button>
    );
  }
  return (
    <Button asChild size="lg" variant="outline">
      <Link to="/showcase">
        {t('audiences.partners.cta')}
        <ArrowRight aria-hidden />
      </Link>
    </Button>
  );
}

/** The last call to start, next to the university's photo and seal. */
function FinalCta({ signedIn }: { signedIn: boolean }) {
  const { t } = useTranslation('landing');
  const tenant = useQuery(tenantQuery);
  const text = useLocalized();
  const cover = tenant.data?.cover_url;
  return (
    <section className="relative isolate overflow-x-clip" aria-labelledby="cta-title">
      <EdgeIllustration
        name="megaphone"
        placement="left"
        className="-z-10 hidden w-60 -translate-y-2/3 -rotate-12 lg:block"
      />
      <EdgeIllustration
        name="rocket"
        placement="right"
        className="-z-10 hidden w-56 translate-y-0 rotate-12 lg:block"
      />
      <div className="mx-auto max-w-page px-4 pb-20 sm:px-6 lg:pb-28">
        <div className="grid grid-cols-1 overflow-hidden rounded-card border border-border bg-surface lg:grid-cols-2">
          <div className="flex flex-col justify-center gap-5 p-8 sm:p-12">
            <Kicker>{t('cta.kicker')}</Kicker>
            <h2
              id="cta-title"
              className="font-display text-3xl font-extrabold text-ink sm:text-5xl"
            >
              {t('cta.title')}
            </h2>
            <p className="text-lg leading-relaxed text-fg-muted">{t('cta.text')}</p>
            <div className="flex flex-wrap gap-3">
              <Button asChild size="lg">
                <Link to={signedIn ? '/app' : '/register'}>
                  {signedIn ? t('actions.toApp') : t('cta.primary')}
                  <ArrowRight aria-hidden />
                </Link>
              </Button>
              {!signedIn && (
                <Button asChild size="lg" variant="secondary">
                  <Link to="/login">{t('actions.signIn')}</Link>
                </Button>
              )}
            </div>
          </div>
          <div className="relative isolate flex min-h-72 items-end bg-deep-900 p-8">
            {cover && (
              <img
                src={cover}
                alt=""
                loading="lazy"
                decoding="async"
                className="absolute inset-0 -z-10 size-full object-cover"
              />
            )}
            <div
              aria-hidden
              className="absolute inset-0 -z-10 bg-linear-to-t from-deep-900/90 via-deep-900/30 to-transparent"
            />
            {tenant.data && (
              <div className="flex items-center gap-4">
                <BrandMark src={tenant.data.logo_url} inverted className="size-14" />
                <div className="flex flex-col">
                  <p className="font-display text-lg leading-snug font-bold text-white">
                    {text(tenant.data.name)}
                  </p>
                  <p className="text-2xs font-bold tracking-label text-primary-300 uppercase">
                    {t('cta.photoCaption')}
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
