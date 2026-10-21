import { useTranslation } from 'react-i18next';

import { i18n, isLocale, type Locale } from './index';

export interface LocalizedText {
  kk?: string;
  ru?: string;
  en?: string;
}

// When the text is missing in the interface language, prefer the other local
// language before English (a Kazakh reader understands Russian and vice versa).
const ORDER: Record<Locale, Locale[]> = {
  kk: ['kk', 'ru', 'en'],
  ru: ['ru', 'kk', 'en'],
  en: ['en', 'ru', 'kk'],
};

/** Picks the best variant of a database text (faculty, university name) for a language. */
export function localized(text: LocalizedText | null | undefined, locale?: string): string {
  if (!text) return '';
  const lang = isLocale(locale) ? locale : isLocale(i18n.language) ? i18n.language : 'kk';
  for (const candidate of ORDER[lang]) {
    const value = text[candidate];
    if (value) return value;
  }
  return '';
}

/** `localized` bound to the current interface language (re-renders on switch). */
export function useLocalized(): (text: LocalizedText | null | undefined) => string {
  const { i18n: instance } = useTranslation();
  return (text) => localized(text, instance.language);
}
