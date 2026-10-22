import { afterEach, describe, expect, it, vi } from 'vitest';

async function freshTheme() {
  vi.resetModules();
  return import('./theme');
}

describe('theme', () => {
  afterEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('is light by default, even when the system is dark', async () => {
    vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string) =>
        ({
          matches: query.includes('dark'),
          media: query,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
        }) as unknown as MediaQueryList,
    );
    const { useTheme } = await freshTheme();
    expect(useTheme.getState().preference).toBe('light');
    expect(useTheme.getState().resolved).toBe('light');
  });

  it('keeps the theme the user chose', async () => {
    localStorage.setItem('qadam.color-theme', 'dark');
    const { useTheme } = await freshTheme();
    expect(useTheme.getState().resolved).toBe('dark');
  });
});
