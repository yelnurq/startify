import { describe, expect, it } from 'vitest';

import { formatDateTime, formatNumber, formatRelative } from './format';
import { detectLocale } from './index';

describe('detectLocale', () => {
  it('prefers the saved choice', () => {
    expect(detectLocale('en', ['ru-RU'])).toBe('en');
  });

  it('uses the first supported browser language', () => {
    expect(detectLocale(null, ['de-DE', 'ru-RU', 'en-US'])).toBe('ru');
    expect(detectLocale(null, ['kk-KZ'])).toBe('kk');
  });

  it('ignores unsupported saved values and falls back to Kazakh', () => {
    expect(detectLocale('fr', ['de-DE'])).toBe('kk');
  });
});

describe('formatters', () => {
  it('formats numbers with locale grouping', () => {
    expect(formatNumber(1234567, 'en')).toBe('1,234,567');
    // Russian and Kazakh group digits with a (narrow) no-break space.
    expect(formatNumber(1234567, 'ru').replace(/\s/g, ' ')).toBe('1 234 567');
  });

  it('formats dates in the interface language', () => {
    const date = new Date(Date.UTC(2026, 9, 21, 12, 0));
    const options = { dateStyle: 'long', timeZone: 'UTC' } as const;
    expect(formatDateTime(date, 'en', options)).toBe('21 October 2026');
    expect(formatDateTime(date, 'ru', options)).toMatch(/21 октября 2026/);
  });

  it('formats relative time with the largest fitting unit', () => {
    const now = new Date('2026-10-21T12:00:00Z');
    expect(formatRelative(new Date('2026-10-21T11:59:58Z'), 'en', now)).toBe('2 seconds ago');
    expect(formatRelative(new Date('2026-10-24T12:00:00Z'), 'en', now)).toBe('in 3 days');
  });
});
