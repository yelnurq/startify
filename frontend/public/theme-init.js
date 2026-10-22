// Runs before the app bundle: Qadam is light (D-052), and it marks the
// language the app will choose, so that a first screen drawn by the server in
// another language stays hidden (src/design-system/base.css). Keep in sync
// with detectLocale in src/shared/i18n (storage key, order, default).
(function () {
  var root = document.documentElement;
  root.dataset.theme = 'light';

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
