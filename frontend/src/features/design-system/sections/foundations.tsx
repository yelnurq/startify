import { Check, X } from 'lucide-react';
import { motion, useReducedMotion } from 'motion/react';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { Badge, Button, cn, contrastRatio, useTheme } from '@/design-system';
import { useFormatters } from '@/shared/i18n/format';

import { Section, Specimen } from '../section';

const STEPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950] as const;
const KAZAKH_ALPHABET =
  'А Ә Б В Г Ғ Д Е Ё Ж З И Й К Қ Л М Н Ң О Ө П Р С Т У Ұ Ү Ф Х Һ Ц Ч Ш Щ Ъ Ы І Ь Э Ю Я';
const KAZAKH_SPECIFIC = 'Ә Ғ Қ Ң Ө Ұ Ү Һ І';

/**
 * Reads CSS custom properties of the active theme. The theme store updates only
 * after the new theme is applied, so reading during render sees fresh values.
 */
function useCssVars(names: readonly string[]): Record<string, string> {
  const theme = useTheme((s) => s.resolved);
  const key = names.join('|');
  return useMemo(() => {
    const style = getComputedStyle(document.documentElement);
    return Object.fromEntries(key.split('|').map((n) => [n, style.getPropertyValue(n).trim()]));
    // The theme is a dependency on purpose: it invalidates the computed values.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, theme]);
}

function ContrastChip({ fg, bg }: { fg: string; bg: string }) {
  const { t } = useTranslation('design');
  const format = useFormatters();
  if (!fg.startsWith('#') || !bg.startsWith('#')) return null;
  const ratio = contrastRatio(fg, bg);
  const ok = ratio >= 4.5;
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-xs font-semibold tabular-nums',
        ok ? 'bg-success-soft text-success-soft-fg' : 'bg-surface-sunken text-fg-muted',
      )}
    >
      {ok ? <Check className="size-3" aria-hidden /> : <X className="size-3" aria-hidden />}
      {ok ? t('colors.aa') : t('colors.decorOnly')}{' '}
      {format.number(ratio, { maximumFractionDigits: 1 })}
    </span>
  );
}

function ScaleRow({ name, label, note }: { name: string; label: string; note: string }) {
  const vars = useCssVars(STEPS.map((s) => `--color-${name}-${s}`));
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h3 className="text-sm font-semibold text-fg">{label}</h3>
        <p className="text-sm text-fg-subtle">{note}</p>
      </div>
      <div className="grid grid-cols-6 gap-1.5 sm:grid-cols-11">
        {STEPS.map((step) => (
          <div key={step} className="flex flex-col gap-1.5">
            <div
              className="aspect-square rounded-lg ring-1 ring-black/5 ring-inset dark:ring-white/5"
              style={{ background: `var(--color-${name}-${step})` }}
            />
            <span className="text-xs font-semibold text-fg tabular-nums">{step}</span>
            <span className="-mt-1 font-mono text-xs text-fg-subtle uppercase">
              {(vars[`--color-${name}-${step}`] ?? '').replace('#', '')}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

const SURFACE_TOKENS = [
  { token: '--qd-bg', key: 'bg', cls: 'bg-bg' },
  { token: '--qd-surface', key: 'surface', cls: 'bg-surface' },
  { token: '--qd-surface-raised', key: 'raised', cls: 'bg-surface-raised' },
  { token: '--qd-surface-sunken', key: 'sunken', cls: 'bg-surface-sunken' },
  { token: '--qd-border', key: 'border', cls: 'bg-border' },
] as const;

const TEXT_TOKENS = [
  { token: '--qd-fg', key: 'fg', cls: 'bg-fg' },
  { token: '--qd-fg-muted', key: 'muted', cls: 'bg-fg-muted' },
  { token: '--qd-fg-subtle', key: 'subtle', cls: 'bg-fg-subtle' },
  { token: '--qd-fg-faint', key: 'faint', cls: 'bg-fg-faint' },
  { token: '--qd-primary-text', key: 'link', cls: 'bg-primary-text' },
] as const;

export function ColorsSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const vars = useCssVars([
    ...SURFACE_TOKENS.map((s) => s.token),
    ...TEXT_TOKENS.map((s) => s.token),
  ]);
  const bg = vars['--qd-surface'] ?? '';
  return (
    <Section id="colors" index={index} title={t('colors.title')} lead={t('colors.lead')}>
      <Specimen label={t('colors.brand')} bodyClassName="flex flex-col gap-8">
        <ScaleRow name="primary" label={t('colors.primaryName')} note={t('colors.primaryNote')} />
        <ScaleRow name="gold" label={t('colors.goldName')} note={t('colors.goldNote')} />
      </Specimen>
      <Specimen label={t('colors.semantic')} bodyClassName="flex flex-col gap-8">
        <ScaleRow name="success" label={t('colors.success')} note={t('colors.successNote')} />
        <ScaleRow name="danger" label={t('colors.danger')} note={t('colors.dangerNote')} />
        <ScaleRow name="warning" label={t('colors.warning')} note={t('colors.warningNote')} />
        <ScaleRow name="info" label={t('colors.info')} note={t('colors.infoNote')} />
      </Specimen>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('colors.surfaces')} bodyClassName="flex flex-col gap-2">
          {SURFACE_TOKENS.map(({ token, key, cls }) => (
            <div key={token} className="flex items-center gap-3">
              <span
                className={cn('size-10 shrink-0 rounded-lg border border-border-strong', cls)}
              />
              <span className="flex-1 text-sm font-medium text-fg">
                {t(`colors.tokens.${key}`)}
              </span>
              <span className="font-mono text-xs text-fg-subtle">{vars[token]}</span>
            </div>
          ))}
        </Specimen>
        <Specimen label={t('colors.text')} bodyClassName="flex flex-col gap-3.5">
          {TEXT_TOKENS.map(({ token, key, cls }) => (
            <div key={token} className="flex items-center gap-3">
              <span className={cn('size-3 shrink-0 rounded-full', cls)} />
              <span className="flex-1 text-sm font-medium text-fg">
                {t(`colors.tokens.${key}`)}
              </span>
              <ContrastChip fg={vars[token] ?? ''} bg={bg} />
            </div>
          ))}
        </Specimen>
      </div>
    </Section>
  );
}

const TEXT_SCALE = [
  { cls: 'text-3xl', px: 30 },
  { cls: 'text-2xl', px: 24 },
  { cls: 'text-xl', px: 20 },
  { cls: 'text-lg', px: 18 },
  { cls: 'text-base', px: 16 },
  { cls: 'text-sm', px: 14 },
  { cls: 'text-xs', px: 12 },
] as const;

export function TypographySection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const format = useFormatters();
  return (
    <Section
      id="typography"
      index={index}
      title={t('typography.title')}
      lead={t('typography.lead')}
    >
      <Specimen label={t('typography.display')} bodyClassName="flex flex-col gap-5">
        <p className="font-display text-4xl font-semibold text-fg sm:text-6xl">
          {t('typography.headline')}
        </p>
        <p className="font-display text-2xl font-medium text-fg-muted sm:text-4xl">
          {t('typography.subhead')}
        </p>
        <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm text-fg-subtle">
          <span>{t('typography.displayMeta')}</span>
        </div>
      </Specimen>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <Specimen label={t('typography.text')} bodyClassName="flex flex-col gap-4">
          {TEXT_SCALE.map(({ cls, px }) => (
            <div key={cls} className="flex items-baseline gap-4">
              <span className="w-10 shrink-0 text-xs text-fg-subtle tabular-nums">{px}</span>
              <p className={cn(cls, 'leading-snug text-fg')}>{t('typography.sample')}</p>
            </div>
          ))}
        </Specimen>
        <Specimen label={t('typography.alphabet')} bodyClassName="flex flex-col gap-5">
          <p
            lang="kk"
            className="font-display text-4xl leading-tight font-semibold tracking-wide text-primary-text"
          >
            {KAZAKH_SPECIFIC}
          </p>
          <p lang="kk" className="text-lg leading-relaxed tracking-wide text-fg">
            {KAZAKH_ALPHABET}
          </p>
          <p lang="kk" className="font-display text-lg leading-relaxed tracking-wide text-fg-muted">
            {KAZAKH_ALPHABET.toLowerCase()}
          </p>
          <p className="text-sm text-fg-subtle">{t('typography.alphabetNote')}</p>
        </Specimen>
      </div>
      <Specimen label={t('typography.numbers')} bodyClassName="grid gap-6 sm:grid-cols-3">
        {[1284, 371, 63.4].map((value) => (
          <div key={value} className="flex flex-col gap-1">
            <span className="font-display text-4xl font-semibold text-fg tabular-nums">
              {format.number(value)}
            </span>
            <span className="text-sm text-fg-subtle">{t('typography.numbersNote')}</span>
          </div>
        ))}
      </Specimen>
    </Section>
  );
}

const RADII = [
  { cls: 'rounded-md', key: 'radiusInput', px: '8' },
  { cls: 'rounded-lg', key: 'radiusButton', px: '12' },
  { cls: 'rounded-xl', key: 'radiusPanel', px: '16' },
  { cls: 'rounded-2xl', key: 'radiusModal', px: '20' },
  { cls: 'rounded-full', key: 'radiusFull', px: '∞' },
] as const;

const SHADOWS = [
  { cls: 'shadow-sm', key: 'shadowSm' },
  { cls: 'shadow-md', key: 'shadowMd' },
  { cls: 'shadow-lg', key: 'shadowLg' },
] as const;

export function ShapeSection({ index }: { index: number }) {
  const { t } = useTranslation('design');
  const reduce = useReducedMotion();
  const [run, setRun] = useState(0);
  return (
    <Section id="shape" index={index} title={t('shape.title')} lead={t('shape.lead')}>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('shape.radii')} bodyClassName="grid grid-cols-5 gap-3">
          {RADII.map(({ cls, key, px }) => (
            <div key={cls} className="flex flex-col items-center gap-2 text-center">
              <div
                className={cn(
                  'aspect-square w-full border-2 border-primary-400 bg-primary-soft',
                  cls,
                )}
              />
              <span className="text-sm font-semibold text-fg tabular-nums">{px}</span>
              <span className="text-xs leading-tight text-fg-subtle">{t(`shape.${key}`)}</span>
            </div>
          ))}
        </Specimen>
        <Specimen label={t('shape.shadows')} bodyClassName="grid grid-cols-3 gap-5 bg-bg">
          {SHADOWS.map(({ cls, key }) => (
            <div
              key={cls}
              className={cn(
                'flex aspect-[4/3] flex-col justify-end rounded-xl border border-border bg-surface p-3',
                cls,
              )}
            >
              <span className="text-sm font-semibold text-fg">{cls.replace('shadow-', '')}</span>
              <span className="text-xs text-fg-subtle">{t(`shape.${key}`)}</span>
            </div>
          ))}
        </Specimen>
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Specimen label={t('shape.spacing')} bodyClassName="flex flex-col gap-2">
          {[1, 2, 3, 4, 6, 8, 12, 16].map((n) => (
            <div key={n} className="flex items-center gap-3">
              <span className="w-12 text-xs text-fg-subtle tabular-nums">{n * 4}</span>
              <span
                className="h-3 rounded-sm bg-primary-300 dark:bg-primary-700"
                style={{ width: n * 4 * 2 }}
              />
            </div>
          ))}
        </Specimen>
        <Specimen label={t('shape.motion')} bodyClassName="flex flex-col gap-4">
          <p className="text-sm text-fg-muted">{t('shape.motionNote')}</p>
          <div className="flex h-24 items-center rounded-lg bg-surface-sunken px-3">
            <motion.div
              key={run}
              initial={reduce ? false : { x: 0, opacity: 0.4, scale: 0.9 }}
              animate={{ x: reduce ? 0 : 'min(60vw, 18rem)', opacity: 1, scale: 1 }}
              transition={{ type: 'spring', stiffness: 260, damping: 22 }}
              className="grid size-14 place-items-center rounded-xl bg-primary text-primary-fg shadow-md"
            >
              <span className="font-display text-lg font-semibold">Q</span>
            </motion.div>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                setRun((n) => n + 1);
              }}
            >
              {t('shape.replay')}
            </Button>
            <Badge tone="info">{t('shape.reducedMotion')}</Badge>
          </div>
        </Specimen>
      </div>
    </Section>
  );
}
