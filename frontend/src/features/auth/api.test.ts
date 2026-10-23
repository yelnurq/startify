import { describe, expect, it } from 'vitest';

import { safeRedirect } from './api';

describe('safeRedirect', () => {
  it('keeps paths of this site', () => {
    for (const path of [
      '/app',
      '/app/projects/1?tab=steps',
      '/showcase#top',
      '/x?u=https://a.b/',
    ]) {
      expect(safeRedirect(path)).toBe(path);
    }
  });

  it('refuses addresses that leave the site', () => {
    for (const target of [
      '//evil.example',
      '/\\evil.example',
      '/app\\..\\..\\evil.example',
      'https://evil.example/',
      'javascript:alert(1)',
      'app',
      '',
      '/\t/evil.example',
      '/\n/evil.example',
      '/\r\n/evil.example',
      '/\u0000/evil.example',
      '/\u0085/evil.example',
      ' /app',
    ]) {
      expect(safeRedirect(target), JSON.stringify(target)).toBeUndefined();
    }
    expect(safeRedirect(undefined)).toBeUndefined();
    expect(safeRedirect(42)).toBeUndefined();
  });
});
