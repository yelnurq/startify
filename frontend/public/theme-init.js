// Runs before the app bundle: Qadam is light (D-052), and it marks the
// language the app will choose, so that a first screen drawn by the server in
// another language stays hidden (src/design-system/base.css). Keep in sync
// with detectLocale in src/shared/i18n (link, storage key, order, default).
(function () {
  var root = document.documentElement;
  root.dataset.theme = 'light';

  var locales = ['kk', 'ru', 'en'];
  var locale = null;
  // A link to one language version (?lang=) chooses the language, and the
  // choice is kept like one made in the menu.
  var match = /[?&]lang=([a-z]+)/.exec(window.location.search);
  if (match && locales.indexOf(match[1]) >= 0) {
    locale = match[1];
    try {
      localStorage.setItem('qadam.locale', locale);
    } catch (e) {
      // Storage unavailable: the link decides for this page only.
    }
  }
  try {
    var saved = localStorage.getItem('qadam.locale');
    if (!locale && locales.indexOf(saved) >= 0) locale = saved;
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
