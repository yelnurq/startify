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

/**
 * Picks the language of the link (`?lang=`, the address of one language
 * version that search engines index), then the saved language, then the
 * first supported browser language, then the default. The server draws the
 * first screen in the same order (backend internal/showcase/boot.go).
 */
export function detectLocale(
  saved: string | null = readSaved(),
  preferred: readonly string[] = typeof navigator === 'undefined' ? [] : navigator.languages,
  requested: string | null = readRequested(),
): Locale {
  if (isLocale(requested)) return requested;
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

function readRequested(): string | null {
  if (typeof window === 'undefined') return null;
  return new URLSearchParams(window.location.search).get('lang');
}

export async function changeLocale(locale: Locale): Promise<void> {
  try {
    localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // Storage unavailable: the choice lasts for this session only.
  }
  await i18n.changeLanguage(locale);
}

// The server draws the first screen of public pages in this language
// (backend internal/showcase/boot.go reads the cookie).
const LOCALE_COOKIE = 'qadam_locale';

i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng;
  document.documentElement.dataset.locale = lng;
  const secure = window.location.protocol === 'https:' ? '; Secure' : '';
  document.cookie = `${LOCALE_COOKIE}=${lng}; Path=/; Max-Age=31536000; SameSite=Lax${secure}`;
});

/** Translation catalogues loaded on demand by the screens that need them. */
export type Namespace =
  | 'common'
  | 'design'
  | 'auth'
  | 'landing'
  | 'onboarding'
  | 'app'
  | 'team'
  | 'admin'
  | 'ai'
  | 'incubator'
  | 'evaluation'
  | 'showcase'
  | 'directory';

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
