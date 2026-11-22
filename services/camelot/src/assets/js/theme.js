// Theme: a switch between light and dark. Until Dhira flips it, the page
// follows the system setting (and its changes); a flip is remembered in
// localStorage. Loaded in <head>, so it runs before paint. Without script
// the system setting applies (the CSS media query).
(function () {
  'use strict';
  var KEY = 'camelot-theme';
  var root = document.documentElement;
  var system = window.matchMedia('(prefers-color-scheme: dark)');

  function saved() {
    try {
      var t = localStorage.getItem(KEY);
      return t === 'dark' || t === 'light' ? t : '';
    } catch (e) {
      return '';
    }
  }

  function current() {
    return saved() || (system.matches ? 'dark' : 'light');
  }

  function apply() {
    var t = current();
    root.classList.toggle('dark', t === 'dark');
    root.classList.toggle('light', t === 'light');
    var b = document.querySelector('[data-theme-toggle]');
    if (!b) return;
    b.hidden = false;
    b.setAttribute('aria-pressed', String(t === 'dark'));
    b.querySelector('[data-theme-name]').textContent = t === 'dark' ? 'Dark' : 'Light';
  }

  apply();
  document.addEventListener('DOMContentLoaded', apply);
  system.addEventListener('change', apply);
  document.addEventListener('click', function (e) {
    if (!e.target.closest || !e.target.closest('[data-theme-toggle]')) return;
    try { localStorage.setItem(KEY, current() === 'dark' ? 'light' : 'dark'); } catch (err) { /* private mode: this page only */ }
    apply();
  });
})();
