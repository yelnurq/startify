// Runs before the app bundle: applies the saved theme without a flash of the
// wrong one, and marks the language the app will choose, so that a first
// screen drawn by the server in another language stays hidden
// (src/design-system/base.css). Keep in sync with src/design-system/theme.ts
// and detectLocale in src/shared/i18n (storage keys, order, default).
(function () {
  var root = document.documentElement;
  var preference = 'system';
  try {
    preference = localStorage.getItem('qadam.theme') || 'system';
  } catch (e) {
    // Storage may be unavailable (private mode); fall back to the system theme.
  }
  var dark =
    preference === 'dark' ||
    (preference === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
  root.dataset.theme = dark ? 'dark' : 'light';

  var locales = ['kk', 'ru', 'en'];
  var locale = null;
  try {
    var saved = localStorage.getItem('qadam.locale');
    if (locales.indexOf(saved) >= 0) locale = saved;
  } catch (e) {
    // No saved language: the browser's languages decide.
  }
  var preferred = navigator.languages || [navigator.language];
  for (var i = 0; !locale && i < preferred.length; i++) {
    var base = String(preferred[i]).toLowerCase().split('-')[0];
    if (locales.indexOf(base) >= 0) locale = base;
  }
  root.dataset.locale = locale || 'kk';
})();
