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

  it('is light, whatever was chosen before and whatever the system prefers', async () => {
    localStorage.setItem('qadam.color-theme', 'dark');
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
    expect(useTheme.getState().resolved).toBe('light');
  });
});
