import { describe, expect, it, vi } from 'vitest';

import { installChunkReload, isChunkLoadError, mayReload } from './chunk-reload';

describe('reload after a deployment', () => {
  it('reloads once, then lets the error show', () => {
    sessionStorage.clear();
    expect(mayReload(sessionStorage, 1_000_000)).toBe(true);
    // The page came back and failed again at once: no reload loop.
    expect(mayReload(sessionStorage, 1_005_000)).toBe(false);
    // Much later it is a new deployment again.
    expect(mayReload(sessionStorage, 1_100_000)).toBe(true);
  });

  it('never reloads without storage to remember it', () => {
    expect(mayReload(undefined)).toBe(false);
    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
    } as unknown as Storage;
    expect(mayReload(broken)).toBe(false);
  });

  it('handles the preload error of Vite by reloading the page', () => {
    sessionStorage.clear();
    const reload = vi.fn();
    const target = new EventTarget() as unknown as Window;
    Object.defineProperty(target, 'location', { value: { reload } });
    installChunkReload(target);

    const first = new Event('vite:preloadError', { cancelable: true });
    target.dispatchEvent(first);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(first.defaultPrevented).toBe(true);

    const second = new Event('vite:preloadError', { cancelable: true });
    target.dispatchEvent(second);
    expect(reload).toHaveBeenCalledTimes(1);
    expect(second.defaultPrevented).toBe(false);
  });

  it('recognises failed chunk loads of the browsers', () => {
    expect(
      isChunkLoadError(new TypeError('Failed to fetch dynamically imported module: /a.js')),
    ).toBe(true);
    expect(isChunkLoadError(new TypeError('Importing a module script failed.'))).toBe(true);
    expect(isChunkLoadError(new Error('error loading dynamically imported module'))).toBe(true);
    expect(isChunkLoadError(new Error('Unable to preload CSS for /a.css'))).toBe(true);
    expect(isChunkLoadError(new Error('x is not a function'))).toBe(false);
    expect(isChunkLoadError('text')).toBe(false);
  });
});
