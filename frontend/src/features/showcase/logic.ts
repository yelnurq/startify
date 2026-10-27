import type { components } from '@/shared/api/schema.gen';
import { compact, optionalString } from '@/shared/router/search';

export type Industry = components['schemas']['Industry'];
export type ShowcaseStage = components['schemas']['ShowcaseStage'];
export type ShowcaseNeed = components['schemas']['ShowcaseNeed'];
export type ContactKind = components['schemas']['ContactKind'];

export const INDUSTRIES = [
  'agritech',
  'edtech',
  'fintech',
  'healthtech',
  'logistics',
  'cleantech',
  'govtech',
  'foodtech',
  'fashion',
  'ecommerce',
  'manufacturing',
  'energy',
  'tourism',
  'social',
  'other',
] as const satisfies readonly Industry[];

export const STAGES = [
  'idea',
  'market',
  'product',
  'launch',
] as const satisfies readonly ShowcaseStage[];

export const NEEDS = [
  'investment',
  'team',
  'partner',
  'pilot',
  'mentor',
] as const satisfies readonly ShowcaseNeed[];

export const CONTACT_KINDS = [
  'investment',
  'partnership',
  'pilot',
  'mentoring',
  'hiring',
  'other',
] as const satisfies readonly ContactKind[];

/** Limits of the page fields, as the API checks them. */
export const LIMITS = {
  slug: 60,
  title: 120,
  tagline: 160,
  video: 500,
  problem: 1500,
  solution: 1500,
  market: 1000,
  traction: 1000,
  ask: 800,
  needs: 5,
  tags: 8,
  tag: 40,
  search: 100,
} as const;

function oneOf<T extends string>(values: readonly T[], value: unknown): T | undefined {
  return typeof value === 'string' && (values as readonly string[]).includes(value)
    ? (value as T)
    : undefined;
}

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** Filters of the showcase as they appear in the address. */
export interface ShowcaseSearch {
  q?: string;
  /** Absent means the university of the address. */
  scope?: 'national';
  industry?: Industry;
  stage?: ShowcaseStage;
  faculty?: string;
  year?: number;
  need?: ShowcaseNeed;
  tag?: string;
}

export type FilterKey = 'industry' | 'stage' | 'faculty' | 'year' | 'need' | 'tag';

export const FILTER_KEYS: readonly FilterKey[] = [
  'industry',
  'stage',
  'faculty',
  'year',
  'need',
  'tag',
];

/** Reads the filters from the address; anything unknown or malformed is dropped. */
export function parseShowcaseSearch(raw: Record<string, unknown>): ShowcaseSearch {
  const q = optionalString(typeof raw.q === 'string' ? raw.q.trim() : raw.q, LIMITS.search);
  const year = typeof raw.year === 'string' ? Number(raw.year) : raw.year;
  const faculty =
    typeof raw.faculty === 'string' && UUID.test(raw.faculty) ? raw.faculty : undefined;
  return compact({
    q,
    scope: raw.scope === 'national' ? ('national' as const) : undefined,
    industry: oneOf(INDUSTRIES, raw.industry),
    stage: oneOf(STAGES, raw.stage),
    faculty,
    year:
      typeof year === 'number' && Number.isInteger(year) && year >= 2000 && year <= 2100
        ? year
        : undefined,
    need: oneOf(NEEDS, raw.need),
    tag: optionalString(raw.tag, LIMITS.tag),
  });
}

/** The query of `GET /showcase` for the filters. */
export function listQuery(search: ShowcaseSearch) {
  return compact({
    scope: search.scope ?? ('university' as const),
    q: search.q,
    industry: search.industry,
    stage: search.stage,
    faculty: search.faculty,
    year: search.year,
    need: search.need,
    tag: search.tag,
  });
}

/** The filters that are set, in the order of the panel (search and scope aside). */
export function activeFilters(search: ShowcaseSearch): { key: FilterKey; value: string }[] {
  const out: { key: FilterKey; value: string }[] = [];
  for (const key of FILTER_KEYS) {
    const value = search[key];
    if (value !== undefined) out.push({ key, value: String(value) });
  }
  return out;
}

/** Sets (or with `undefined` clears) one filter, keeping the others. */
export function withFilter<K extends FilterKey>(
  search: ShowcaseSearch,
  key: K,
  value: ShowcaseSearch[K] | undefined,
): ShowcaseSearch {
  return compact({ ...search, [key]: value });
}

/** The search and scope without any filter. */
export function withoutFilters(search: ShowcaseSearch): ShowcaseSearch {
  return compact({ q: search.q, scope: search.scope });
}

/** Facets of another university do not apply to the national scope (faculties). */
export function switchScope(search: ShowcaseSearch, national: boolean): ShowcaseSearch {
  const { scope: _scope, faculty: _faculty, ...rest } = search;
  return national ? { ...rest, scope: 'national' } : rest;
}

export const SLUG_PATTERN = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export function isValidSlug(slug: string): boolean {
  return slug.length >= 3 && slug.length <= LIMITS.slug && SLUG_PATTERN.test(slug);
}

// Kazakh and Russian letters in Latin, close to the official Kazakh Latin alphabet.
const TRANSLIT: Record<string, string> = {
  а: 'a',
  ә: 'a',
  б: 'b',
  в: 'v',
  г: 'g',
  ғ: 'g',
  д: 'd',
  е: 'e',
  ё: 'e',
  ж: 'zh',
  з: 'z',
  и: 'i',
  й: 'i',
  к: 'k',
  қ: 'q',
  л: 'l',
  м: 'm',
  н: 'n',
  ң: 'n',
  о: 'o',
  ө: 'o',
  п: 'p',
  р: 'r',
  с: 's',
  т: 't',
  у: 'u',
  ұ: 'u',
  ү: 'u',
  ф: 'f',
  х: 'h',
  һ: 'h',
  ц: 'ts',
  ч: 'ch',
  ш: 'sh',
  щ: 'sch',
  ъ: '',
  ы: 'y',
  і: 'i',
  ь: '',
  э: 'e',
  ю: 'yu',
  я: 'ya',
  // Kazakh Latin letters without an ASCII decomposition.
  ı: 'i',
};

/** A page address from a title in any of the three languages: "Kiiz Lab" → "kiiz-lab". */
export function slugify(text: string): string {
  const latin = Array.from(text.normalize('NFKD').toLowerCase())
    .map((ch) => TRANSLIT[ch] ?? ch)
    .join('')
    // Accents split off by NFKD (é → e + accent), then everything that is not a letter or digit.
    .replace(/\p{M}/gu, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return latin.slice(0, LIMITS.slug).replace(/-+$/, '');
}

export interface VideoEmbed {
  provider: 'youtube' | 'vimeo';
  /** Address for an iframe. */
  src: string;
}

const YOUTUBE_ID = /^[A-Za-z0-9_-]{11}$/;
const VIMEO_ID = /^\d{1,12}$/;

/**
 * The embeddable player of a video link. Only YouTube (through the privacy
 * enhanced youtube-nocookie.com) and Vimeo are embedded — the CSP allows no
 * other frames; anything else is shown as a plain link.
 */
export function videoEmbed(url: string | null | undefined): VideoEmbed | null {
  if (!url) return null;
  let parsed: URL;
  try {
    parsed = new URL(url.trim());
  } catch {
    return null;
  }
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return null;
  const host = parsed.hostname.toLowerCase().replace(/^(www|m)\./, '');
  const parts = parsed.pathname.split('/').filter(Boolean);
  let youtube: string | undefined;
  if (host === 'youtu.be') youtube = parts[0];
  else if (host === 'youtube.com' || host === 'youtube-nocookie.com') {
    if (parts[0] === 'watch') youtube = parsed.searchParams.get('v') ?? undefined;
    else if (parts[0] === 'embed' || parts[0] === 'shorts' || parts[0] === 'live')
      youtube = parts[1];
  }
  if (youtube !== undefined) {
    return YOUTUBE_ID.test(youtube)
      ? { provider: 'youtube', src: `https://www.youtube-nocookie.com/embed/${youtube}` }
      : null;
  }
  let vimeo: string | undefined;
  if (host === 'vimeo.com') vimeo = parts[0];
  else if (host === 'player.vimeo.com' && parts[0] === 'video') vimeo = parts[1];
  if (vimeo !== undefined && VIMEO_ID.test(vimeo)) {
    return { provider: 'vimeo', src: `https://player.vimeo.com/video/${vimeo}` };
  }
  return null;
}

/** Whether a link may be shown at all (an http(s) address). */
export function isWebLink(url: string | null | undefined): url is string {
  if (!url) return false;
  try {
    const { protocol } = new URL(url);
    return protocol === 'https:' || protocol === 'http:';
  } catch {
    return false;
  }
}

/** Addresses of the share buttons. */
export function shareLinks(url: string, text: string): { telegram: string; whatsapp: string } {
  const u = encodeURIComponent(url);
  return {
    telegram: `https://t.me/share/url?url=${u}&text=${encodeURIComponent(text)}`,
    whatsapp: `https://wa.me/?text=${encodeURIComponent(`${text} ${url}`)}`,
  };
}

/** One or two letters for a project without a cover: "Kiiz Lab" → "KL". */
export function monogram(title: string): string {
  const words = title
    .trim()
    .split(/[\s\-–—_]+/)
    .filter((w) => /\p{L}|\p{N}/u.test(w));
  const letters = words.slice(0, 2).map((w) => Array.from(w.replace(/^[^\p{L}\p{N}]+/u, ''))[0]);
  return letters.join('').toUpperCase();
}

/** The first `max` items and how many are left over ("+2"). */
export function splitVisible<T>(items: readonly T[], max: number): { visible: T[]; rest: number } {
  return { visible: items.slice(0, max), rest: Math.max(items.length - max, 0) };
}

export type SectionKey = 'problem' | 'solution' | 'market' | 'team' | 'traction' | 'ask';

/** Sections of a project page that have something to show, in reading order. */
export function presentSections(page: {
  sections: Partial<Record<'problem' | 'solution' | 'market' | 'traction' | 'ask', string>>;
  team: readonly unknown[];
  needs: readonly unknown[];
}): SectionKey[] {
  const has = (text: string | undefined) => Boolean(text?.trim());
  const out: SectionKey[] = [];
  if (has(page.sections.problem)) out.push('problem');
  if (has(page.sections.solution)) out.push('solution');
  if (has(page.sections.market)) out.push('market');
  if (page.team.length > 0) out.push('team');
  if (has(page.sections.traction)) out.push('traction');
  if (has(page.sections.ask) || page.needs.length > 0) out.push('ask');
  return out;
}

export interface FormMerge<T> {
  /** The values to keep editing: others' changes taken, own changes kept. */
  merged: T;
  /** Fields both sides changed differently since the common version. */
  conflicts: (keyof T & string)[];
}

/**
 * Three-way merge of a flat form by field, as the step editor merges a step
 * (D-036): `base` is the version both sides started from, `mine` the local
 * edits, `theirs` the version on the server. A field both sides changed
 * differently keeps the local value and is reported.
 */
export function mergeForm<T extends object>(base: T, mine: T, theirs: T): FormMerge<T> {
  const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);
  const merged = { ...theirs };
  const conflicts: (keyof T & string)[] = [];
  for (const key of Object.keys(theirs) as (keyof T & string)[]) {
    if (same(mine[key], base[key]) || same(mine[key], theirs[key])) {
      merged[key] = theirs[key];
    } else if (same(theirs[key], base[key])) {
      merged[key] = mine[key];
    } else {
      merged[key] = mine[key];
      conflicts.push(key);
    }
  }
  return { merged, conflicts };
}

/** Text cut at a word boundary with an ellipsis, for short previews. */
export function excerpt(text: string | undefined, max: number): string {
  const clean = (text ?? '').replace(/\s+/g, ' ').trim();
  if (clean.length <= max) return clean;
  const cut = clean.slice(0, max);
  const space = cut.lastIndexOf(' ');
  return `${(space > max * 0.6 ? cut.slice(0, space) : cut).replace(/[\s,.;:–—-]+$/, '')}…`;
}
