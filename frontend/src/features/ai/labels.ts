import type { TFunction } from 'i18next';

const KNOWN_PROVIDERS = ['mock', 'anthropic', 'openai'] as const;

/** Readable name of a provider; providers added by configuration show as named. */
export function providerLabel(t: TFunction<'ai'>, name: string): string {
  return (KNOWN_PROVIDERS as readonly string[]).includes(name)
    ? t(`providers.${name as (typeof KNOWN_PROVIDERS)[number]}`)
    : name;
}
