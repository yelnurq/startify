// Runs before the app bundle to avoid a flash of the wrong theme.
// Keep in sync with src/design-system/theme.ts (storage key and attribute).
(function () {
  var preference = 'system';
  try {
    preference = localStorage.getItem('qadam.theme') || 'system';
  } catch (e) {
    // Storage may be unavailable (private mode); fall back to the system theme.
  }
  var dark =
    preference === 'dark' ||
    (preference === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  document.documentElement.dataset.theme = dark ? 'dark' : 'light';
})();
