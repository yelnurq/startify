import { describe, expect, it } from 'vitest';

import {
  activeFilters,
  excerpt,
  isValidSlug,
  isWebLink,
  listQuery,
  monogram,
  parseShowcaseSearch,
  presentSections,
  shareLinks,
  slugify,
  splitVisible,
  switchScope,
  videoEmbed,
  withFilter,
  withoutFilters,
} from './logic';

const FACULTY = '0192f5d4-7c1e-7a3b-9d2e-4f6a8b0c1d2e';

describe('showcase filters in the address', () => {
  it('keeps known filters and drops malformed ones', () => {
    expect(
      parseShowcaseSearch({
        q: '  агро  ',
        scope: 'national',
        industry: 'agritech',
        stage: 'product',
        faculty: FACULTY,
        year: '2026',
        need: 'pilot',
        tag: 'IoT',
      }),
    ).toEqual({
      q: 'агро',
      scope: 'national',
      industry: 'agritech',
      stage: 'product',
      faculty: FACULTY,
      year: 2026,
      need: 'pilot',
      tag: 'IoT',
    });
    expect(
      parseShowcaseSearch({
        q: '',
        scope: 'university',
        industry: 'spaceflight',
        stage: 'growth',
        faculty: 'not-a-uuid',
        year: '1999',
        need: 'money',
        tag: 'x'.repeat(41),
      }),
    ).toEqual({});
    expect(parseShowcaseSearch({ year: 2026.5 })).toEqual({});
    expect(parseShowcaseSearch({ q: 'a'.repeat(101) })).toEqual({});
  });

  it('round-trips through the API query with the default scope', () => {
    const search = parseShowcaseSearch({ industry: 'edtech', need: 'team' });
    expect(listQuery(search)).toEqual({ scope: 'university', industry: 'edtech', need: 'team' });
    expect(listQuery({ scope: 'national', q: 'qr' })).toEqual({ scope: 'national', q: 'qr' });
  });

  it('lists, sets and clears filters without touching the search', () => {
    const search = { q: 'qr', industry: 'edtech' as const, year: 2025 };
    expect(activeFilters(search)).toEqual([
      { key: 'industry', value: 'edtech' },
      { key: 'year', value: '2025' },
    ]);
    expect(withFilter(search, 'stage', 'idea')).toEqual({ ...search, stage: 'idea' });
    expect(withFilter(search, 'industry', undefined)).toEqual({ q: 'qr', year: 2025 });
    expect(withoutFilters({ ...search, scope: 'national' })).toEqual({
      q: 'qr',
      scope: 'national',
    });
  });

  it('drops the faculty when switching to the whole country', () => {
    expect(switchScope({ faculty: FACULTY, stage: 'idea' }, true)).toEqual({
      scope: 'national',
      stage: 'idea',
    });
    expect(switchScope({ scope: 'national', stage: 'idea' }, false)).toEqual({ stage: 'idea' });
  });
});

describe('page address', () => {
  it('transliterates Kazakh and Russian titles', () => {
    expect(slugify('Kiiz Lab')).toBe('kiiz-lab');
    expect(slugify('Ақылды егіс — AgroScan!')).toBe('aqyldy-egis-agroscan');
    expect(slugify('Қазақша оқу ботı')).toBe('qazaqsha-oqu-boti');
    expect(slugify('Умный щит «Жүйе»')).toBe('umnyi-schit-zhuie');
    expect(slugify('Café № 1')).toBe('cafe-no-1');
    expect(slugify('  --  ')).toBe('');
  });

  it('keeps addresses within 60 characters without a trailing hyphen', () => {
    const slug = slugify(`${'abc '.repeat(14)}defghij`);
    expect(slug.length).toBeLessThanOrEqual(60);
    expect(slug.endsWith('-')).toBe(false);
    expect(isValidSlug(slug)).toBe(true);
  });

  it('validates addresses like the API', () => {
    expect(isValidSlug('agroscan')).toBe(true);
    expect(isValidSlug('kiiz-lab-2')).toBe(true);
    expect(isValidSlug('ab')).toBe(false);
    expect(isValidSlug('Kiiz-Lab')).toBe(false);
    expect(isValidSlug('kiiz--lab')).toBe(false);
    expect(isValidSlug('-kiiz')).toBe(false);
    expect(isValidSlug('a'.repeat(61))).toBe(false);
  });
});

describe('video embeds', () => {
  it('embeds YouTube through youtube-nocookie', () => {
    const src = 'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ';
    for (const url of [
      'https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42',
      'https://youtu.be/dQw4w9WgXcQ',
      'https://m.youtube.com/watch?v=dQw4w9WgXcQ',
      'https://www.youtube.com/embed/dQw4w9WgXcQ',
      'https://youtube.com/shorts/dQw4w9WgXcQ',
      'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ',
    ]) {
      expect(videoEmbed(url)).toEqual({ provider: 'youtube', src });
    }
  });

  it('embeds Vimeo through its player', () => {
    expect(videoEmbed('https://vimeo.com/76979871')).toEqual({
      provider: 'vimeo',
      src: 'https://player.vimeo.com/video/76979871',
    });
    expect(videoEmbed('https://player.vimeo.com/video/76979871?h=abc')?.src).toBe(
      'https://player.vimeo.com/video/76979871',
    );
  });

  it('embeds nothing else', () => {
    expect(videoEmbed('https://rutube.ru/video/abc/')).toBeNull();
    expect(videoEmbed('https://evil.example/youtube.com/watch?v=dQw4w9WgXcQ')).toBeNull();
    expect(videoEmbed('https://youtube.com.evil.example/watch?v=dQw4w9WgXcQ')).toBeNull();
    expect(videoEmbed('https://www.youtube.com/watch?v=short')).toBeNull();
    expect(videoEmbed('https://vimeo.com/channels/staffpicks')).toBeNull();
    expect(videoEmbed('javascript:alert(1)')).toBeNull();
    expect(videoEmbed('not a url')).toBeNull();
    expect(videoEmbed(null)).toBeNull();
  });

  it('shows only web links', () => {
    expect(isWebLink('https://rutube.ru/video/abc/')).toBe(true);
    expect(isWebLink('javascript:alert(1)')).toBe(false);
    expect(isWebLink('')).toBe(false);
  });
});

describe('cards and pages', () => {
  it('builds share links', () => {
    const links = shareLinks('https://kazutb.qadam.kz/showcase/agroscan', 'AgroScan — витрина');
    expect(links.telegram).toBe(
      'https://t.me/share/url?url=https%3A%2F%2Fkazutb.qadam.kz%2Fshowcase%2Fagroscan&text=AgroScan%20%E2%80%94%20%D0%B2%D0%B8%D1%82%D1%80%D0%B8%D0%BD%D0%B0',
    );
    expect(decodeURIComponent(links.whatsapp.split('text=')[1] ?? '')).toBe(
      'AgroScan — витрина https://kazutb.qadam.kz/showcase/agroscan',
    );
  });

  it('makes monograms of titles', () => {
    expect(monogram('Kiiz Lab')).toBe('KL');
    expect(monogram('AgroScan')).toBe('A');
    expect(monogram('«Өрнек» — студия')).toBe('ӨС');
    expect(monogram('')).toBe('');
  });

  it('splits badges into visible and the rest', () => {
    expect(splitVisible(['a', 'b', 'c', 'd'], 2)).toEqual({ visible: ['a', 'b'], rest: 2 });
    expect(splitVisible(['a'], 3)).toEqual({ visible: ['a'], rest: 0 });
  });

  it('lists only the sections with content', () => {
    expect(
      presentSections({
        sections: { problem: 'Боль', solution: ' ', market: 'Рынок', ask: '' },
        team: [{}],
        needs: [],
      }),
    ).toEqual(['problem', 'market', 'team']);
    expect(presentSections({ sections: {}, team: [], needs: ['pilot'] })).toEqual(['ask']);
  });

  it('cuts previews at a word', () => {
    expect(excerpt('Короткий текст', 40)).toBe('Короткий текст');
    expect(excerpt('Фермеры теряют до трети урожая, потому что болезни замечают поздно', 40)).toBe(
      'Фермеры теряют до трети урожая, потому…',
    );
  });
});
