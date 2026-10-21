import { useTranslation } from 'react-i18next';

import { DEFAULT_LOCALE, isLocale, type Locale } from './index';

// English dates follow the day-month order used in Kazakhstan.
const INTL_LOCALE: Record<Locale, string> = { kk: 'kk-KZ', ru: 'ru-RU', en: 'en-GB' };

export function intlLocale(locale: Locale): string {
  return INTL_LOCALE[locale];
}

export function formatNumber(
  value: number,
  locale: Locale,
  options?: Intl.NumberFormatOptions,
): string {
  return new Intl.NumberFormat(intlLocale(locale), options).format(value);
}

export function formatDateTime(
  value: Date | string,
  locale: Locale,
  options: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' },
): string {
  const date = typeof value === 'string' ? new Date(value) : value;
  return new Intl.DateTimeFormat(intlLocale(locale), options).format(date);
}

const RELATIVE_STEPS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ['second', 60],
  ['minute', 60],
  ['hour', 24],
  ['day', 7],
  ['week', 4.34524],
  ['month', 12],
  ['year', Number.POSITIVE_INFINITY],
];

/** "2 секунды назад", "через 3 дня": picks the largest unit that fits. */
export function formatRelative(
  value: Date | string,
  locale: Locale,
  now: Date = new Date(),
): string {
  const date = typeof value === 'string' ? new Date(value) : value;
  let delta = (date.getTime() - now.getTime()) / 1000;
  const rtf = new Intl.RelativeTimeFormat(intlLocale(locale), { numeric: 'auto' });
  for (const [unit, size] of RELATIVE_STEPS) {
    if (Math.abs(delta) < size) return rtf.format(Math.round(delta), unit);
    delta /= size;
  }
  return rtf.format(Math.round(delta), 'year');
}

/** Formatters bound to the current interface language. */
export function useFormatters() {
  const { i18n } = useTranslation();
  const locale = isLocale(i18n.language) ? i18n.language : DEFAULT_LOCALE;
  return {
    locale,
    number: (value: number, options?: Intl.NumberFormatOptions) =>
      formatNumber(value, locale, options),
    dateTime: (value: Date | string, options?: Intl.DateTimeFormatOptions) =>
      formatDateTime(value, locale, options),
    relative: (value: Date | string) => formatRelative(value, locale),
  };
}
