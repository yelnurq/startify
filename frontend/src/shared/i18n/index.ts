import i18n from 'i18next';
import resourcesToBackend from 'i18next-resources-to-backend';
import { initReactI18next } from 'react-i18next';

export const LOCALES = ['kk', 'ru', 'en'] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = 'kk';
export const NAMESPACES = ['common'] as const;

const STORAGE_KEY = 'qadam.locale';

export function isLocale(value: unknown): value is Locale {
  return typeof value === 'string' && (LOCALES as readonly string[]).includes(value);
}

/** Picks the saved language, then the first supported browser language, then the default. */
export function detectLocale(
  saved: string | null = readSaved(),
  preferred: readonly string[] = typeof navigator === 'undefined' ? [] : navigator.languages,
): Locale {
  if (isLocale(saved)) return saved;
  for (const tag of preferred) {
    const base = tag.toLowerCase().split('-')[0];
    if (isLocale(base)) return base;
  }
  return DEFAULT_LOCALE;
}

function readSaved(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return null;
  }
}

export async function changeLocale(locale: Locale): Promise<void> {
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Storage unavailable: the choice lasts for this session only.
  }
  await i18n.changeLanguage(locale);
}

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng;
});

/** Translation catalogues loaded on demand by the screens that need them. */
export type Namespace =
  'common' | 'design' | 'auth' | 'landing' | 'onboarding' | 'app' | 'team' | 'admin' | 'ai';

/** Loads namespaces before a route renders, so no screen flashes raw keys. */
export async function loadNamespaces(...namespaces: Namespace[]): Promise<void> {
  await i18n.loadNamespaces(namespaces);
}

export const i18nReady = i18n
  .use(initReactI18next)
  .use(
    resourcesToBackend(
      (lng: string, ns: string) => import(`../../locales/${lng}/${ns}.json`) as Promise<unknown>,
    ),
  )
  .init({
    lng: detectLocale(),
    supportedLngs: LOCALES,
    // No silent fallback: a Kazakh screen must never show Russian or English
    // strings. `npm run i18n:check` guarantees that every key exists everywhere.
    fallbackLng: false,
    ns: NAMESPACES,
    defaultNS: 'common',
    interpolation: { escapeValue: false },
    returnNull: false,
  });

export { i18n };
