const RELOADED_AT_KEY = 'qadam.chunkReloadAt';
/** A second failure this soon after a reload is not an outdated page: show the error. */
const LOOP_GUARD_MS = 30_000;

/**
 * True when the page may reload to pick up a new release, at most once per
 * guard period; the moment is remembered across the reload.
 */
export function mayReload(storage: Storage | undefined, now = Date.now()): boolean {
  try {
    if (!storage) return false;
    const last = Number(storage.getItem(RELOADED_AT_KEY) ?? 0);
    if (now - last < LOOP_GUARD_MS) return false;
    storage.setItem(RELOADED_AT_KEY, String(now));
    return true;
  } catch {
    // Without storage a reload loop could not be stopped: let the error page explain.
    return false;
  }
}

function sessionStore(): Storage | undefined {
  try {
    return window.sessionStorage;
  } catch {
    return undefined;
  }
}

/**
 * After a deployment the code of the open page may be gone from the server:
 * loading a screen then fails. The page reloads once to get the new release;
 * if that does not help, the error reaches the route's error page.
 */
export function installChunkReload(target: Window = window): void {
  target.addEventListener('vite:preloadError', (event) => {
    if (!mayReload(sessionStore())) return;
    event.preventDefault();
    target.location.reload();
  });
}

/** An error of loading the code of a screen (a chunk missing after a deployment). */
export function isChunkLoadError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return /dynamically imported module|importing a module script failed|unable to preload css|loading (css )?chunk/i.test(
    error.message,
  );
}
