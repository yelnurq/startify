/**
 * Removes the first screen drawn by the server on public pages (#boot, see
 * backend internal/showcase/boot.go) once the app has drawn the page behind
 * it, so that the swap is not visible: `ready` must hold for a few frames in
 * a row (queries start only when the page mounts). A first screen that is
 * hidden (another language) goes at once.
 */
export function releaseBoot(ready: () => boolean, timeoutMs = 10_000): void {
  const boot = document.getElementById('boot');
  if (!boot) return;
  if (getComputedStyle(boot).display === 'none') {
    boot.remove();
    return;
  }
  const started = performance.now();
  let streak = 0;
  const check = () => {
    streak = ready() ? streak + 1 : 0;
    if (streak >= 3 || performance.now() - started > timeoutMs) {
      boot.remove();
      return;
    }
    requestAnimationFrame(check);
  };
  requestAnimationFrame(check);
}
