import { afterEach, describe, expect, it, vi } from 'vitest';

import { releaseBoot } from './boot';

function addBoot(): HTMLElement {
  const boot = document.createElement('div');
  boot.id = 'boot';
  document.body.append(boot);
  return boot;
}

describe('releaseBoot', () => {
  afterEach(() => {
    document.body.innerHTML = '';
    vi.useRealTimers();
  });

  it('keeps the first screen until the page is ready', () => {
    vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'performance'] });
    const boot = addBoot();
    let ready = false;
    releaseBoot(() => ready);
    vi.advanceTimersByTime(500);
    expect(boot.isConnected).toBe(true);
    ready = true;
    vi.advanceTimersByTime(100);
    expect(boot.isConnected).toBe(false);
  });

  it('wants the page ready for a few frames in a row', () => {
    vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'performance'] });
    const boot = addBoot();
    // Ready, then queries start, then ready again.
    const answers = [true, false, true, true];
    releaseBoot(() => answers.shift() ?? true);
    vi.advanceTimersByTime(16 * 4);
    expect(boot.isConnected).toBe(true);
    vi.advanceTimersByTime(16 * 3);
    expect(boot.isConnected).toBe(false);
  });

  it('gives up waiting after the timeout', () => {
    vi.useFakeTimers({ toFake: ['requestAnimationFrame', 'performance'] });
    const boot = addBoot();
    releaseBoot(() => false, 1000);
    vi.advanceTimersByTime(900);
    expect(boot.isConnected).toBe(true);
    vi.advanceTimersByTime(200);
    expect(boot.isConnected).toBe(false);
  });

  it('removes a hidden first screen (another language) at once', () => {
    const boot = addBoot();
    boot.style.display = 'none';
    releaseBoot(() => false);
    expect(boot.isConnected).toBe(false);
  });

  it('does nothing on pages without one', () => {
    expect(() => {
      releaseBoot(() => true);
    }).not.toThrow();
  });
});
