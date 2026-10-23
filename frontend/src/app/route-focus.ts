/** How long a new page may take to draw its heading (data arrives after the route). */
const HEADING_WAIT_MS = 3_000;

/**
 * Moves focus to the main heading of the page just opened, once it is drawn,
 * so that screen readers and the keyboard start there. Calls `missing` when
 * no heading shows up in time. Focus the person moved themselves (into a
 * field, a dialog) is left alone. Returns a function that stops waiting.
 */
export function focusMainHeading(missing: () => void, root: Document = document): () => void {
  const started = root.activeElement;
  const settle = (): boolean => {
    const heading = root.querySelector<HTMLElement>('main h1');
    if (!heading) return false;
    const active = root.activeElement;
    const moved =
      active !== null && active !== root.body && active !== started && active.isConnected;
    if (!moved) {
      if (!heading.hasAttribute('tabindex')) heading.tabIndex = -1;
      heading.dataset.routeFocus = '';
      heading.focus({ preventScroll: true });
    }
    return true;
  };
  if (settle()) return () => undefined;
  const observer = new MutationObserver(() => {
    if (settle()) stop();
  });
  const timer = setTimeout(() => {
    stop();
    missing();
  }, HEADING_WAIT_MS);
  function stop() {
    observer.disconnect();
    clearTimeout(timer);
  }
  observer.observe(root.body, { childList: true, subtree: true });
  return stop;
}
