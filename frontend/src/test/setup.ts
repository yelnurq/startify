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

// jsdom has no ResizeObserver; Radix controls (checkbox, switch) measure themselves with it.
Object.defineProperty(window, 'ResizeObserver', {
  writable: true,
  value: class {
    observe(): void {
      // jsdom does not lay out, so there is nothing to observe.
    }
    unobserve(): void {
      // Nothing observed.
    }
    disconnect(): void {
      // Nothing observed.
    }
  },
});

// The router restores the scroll position on navigation; jsdom does not scroll.
window.scrollTo = () => undefined;

// Components render real Russian strings: the same catalogues as the app.
localStorage.setItem('qadam.locale', 'ru');
const { i18nReady } = await import('@/shared/i18n');
await i18nReady;
// Validation messages speak the interface language, as in main.tsx.
const { installZodLocalization } = await import('@/shared/forms/validation');
installZodLocalization();

afterEach(() => {
  cleanup();
});
