import { Link } from '@tanstack/react-router';
import { Command as CommandIcon, Sparkles } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { PublicLayout } from '@/app/layouts/public-layout';
import { Badge, Button, cn, Kbd, Logo, modKey, useCommandPalette } from '@/design-system';

import { DesignFooter } from './design-footer';
import { FeedbackSection, OverlaysSection } from './sections/behaviour';
import { ButtonsSection, FormsSection } from './sections/controls';
import { CardsSection, IdentitySection } from './sections/display';
import { ColorsSection, ShapeSection, TypographySection } from './sections/foundations';
import {
  DataSection,
  EmptyStatesSection,
  LayoutsSection,
  NavigationSection,
} from './sections/structure';

const SECTIONS = [
  ['colors', ColorsSection],
  ['typography', TypographySection],
  ['shape', ShapeSection],
  ['buttons', ButtonsSection],
  ['forms', FormsSection],
  ['identity', IdentitySection],
  ['cards', CardsSection],
  ['overlays', OverlaysSection],
  ['feedback', FeedbackSection],
  ['navigation', NavigationSection],
  ['data', DataSection],
  ['empty', EmptyStatesSection],
  ['layouts', LayoutsSection],
] as const;

type SectionId = (typeof SECTIONS)[number][0];

/** Highlights the table-of-contents entry of the section in the middle of the viewport. */
function useActiveSection(): SectionId {
  const [active, setActive] = useState<SectionId>('colors');
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActive(entry.target.id as SectionId);
        }
      },
      { rootMargin: '-35% 0px -60% 0px' },
    );
    for (const [id] of SECTIONS) {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    }
    return () => {
      observer.disconnect();
    };
  }, []);
  return active;
}

function Hero() {
  const { t } = useTranslation('design');
  const openPalette = useCommandPalette((s) => s.setOpen);
  const stats = [
    { value: '130+', label: t('hero.stats.tokens') },
    { value: '38', label: t('hero.stats.components') },
    { value: '3', label: t('hero.stats.languages') },
    { value: 'AA', label: t('hero.stats.contrast') },
  ];
  return (
    <div className="relative overflow-hidden border-b border-border">
      <div aria-hidden className="absolute inset-0 ornament-fade opacity-[0.06]" />
      <div
        aria-hidden
        className="absolute -top-48 left-1/2 h-[28rem] w-[64rem] -translate-x-1/2 rounded-full bg-primary-300/30 blur-3xl dark:bg-primary-600/15"
      />
      <div
        aria-hidden
        className="absolute top-16 right-[6%] size-48 rounded-full bg-gold-300/35 blur-3xl dark:bg-gold-500/10"
      />
      <div className="relative mx-auto flex max-w-page flex-col items-start gap-6 px-4 py-16 sm:px-6 lg:py-24">
        <Badge tone="primary" size="lg" icon={<Sparkles />}>
          {t('hero.eyebrow')}
        </Badge>
        <h1 className="max-w-4xl font-display text-4xl font-semibold text-fg sm:text-6xl">
          {t('hero.title')}
        </h1>
        <p className="max-w-2xl text-lg leading-relaxed text-fg-muted">{t('hero.lead')}</p>
        <div className="flex flex-wrap gap-3">
          <Button size="lg" asChild>
            <a href="#buttons">{t('hero.toComponents')}</a>
          </Button>
          <Button
            size="lg"
            variant="secondary"
            leftIcon={<CommandIcon />}
            onClick={() => {
              openPalette(true);
            }}
          >
            {t('hero.openPalette')}
            <span className="ml-1 flex gap-0.5 pointer-coarse:hidden">
              <Kbd>{modKey()}</Kbd>
              <Kbd>K</Kbd>
            </span>
          </Button>
        </div>
        <dl className="mt-4 grid w-full max-w-3xl grid-cols-2 gap-x-8 gap-y-5 sm:grid-cols-4">
          {stats.map((stat) => (
            <div
              key={stat.label}
              className="flex flex-col gap-1 border-l-2 border-primary-300 pl-4 dark:border-primary-700"
            >
              <dt className="order-2 text-sm text-fg-subtle">{stat.label}</dt>
              <dd className="order-1 font-display text-3xl font-semibold text-fg">{stat.value}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}

export function DesignPage() {
  const { t } = useTranslation(['design', 'common']);
  const active = useActiveSection();
  return (
    <PublicLayout
      logo={
        <Link to="/" className="rounded-lg">
          <Logo />
        </Link>
      }
      footer={<DesignFooter />}
    >
      <Hero />
      <nav
        aria-label={t('design:toc.label')}
        className="sticky top-16 z-30 border-b border-border surface-glass lg:hidden"
      >
        <ul className="flex [scrollbar-width:none] gap-1 overflow-x-auto px-4 py-2">
          {SECTIONS.map(([id]) => (
            <li key={id}>
              <a
                href={`#${id}`}
                aria-current={active === id ? 'true' : undefined}
                className="block rounded-full px-3 py-1.5 text-sm whitespace-nowrap text-fg-muted aria-[current]:bg-primary-soft aria-[current]:font-medium aria-[current]:text-primary-soft-fg"
              >
                {t(`design:toc.${id}`)}
              </a>
            </li>
          ))}
        </ul>
      </nav>
      <div className="mx-auto grid max-w-page grid-cols-1 gap-10 px-4 py-12 sm:px-6 lg:grid-cols-[12rem_minmax(0,1fr)] lg:gap-14 lg:py-16">
        <nav aria-label={t('design:toc.label')} className="hidden lg:block">
          <ul className="sticky top-24 flex flex-col gap-0.5 border-l border-border">
            {SECTIONS.map(([id]) => (
              <li key={id}>
                <a
                  href={`#${id}`}
                  aria-current={active === id ? 'true' : undefined}
                  className={cn(
                    '-ml-px block border-l-2 border-transparent py-1.5 pl-4 text-sm text-fg-subtle transition-colors hover:text-fg',
                    'aria-[current]:border-primary aria-[current]:font-medium aria-[current]:text-fg',
                  )}
                >
                  {t(`design:toc.${id}`)}
                </a>
              </li>
            ))}
          </ul>
        </nav>
        <div className="flex min-w-0 flex-col gap-20">
          {SECTIONS.map(([id, Component], i) => (
            <Component key={id} index={i + 1} />
          ))}
        </div>
      </div>
    </PublicLayout>
  );
}
