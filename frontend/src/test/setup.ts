import '@testing-library/jest-dom/vitest';

import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// jsdom has no matchMedia; the theme store and motion query it at import time.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }) as unknown as MediaQueryList,
});

// Components render real Russian strings: the same catalogues as the app.
localStorage.setItem('qadam.locale', 'ru');
const { i18nReady } = await import('@/shared/i18n');
await i18nReady;

afterEach(() => {
  cleanup();
});
