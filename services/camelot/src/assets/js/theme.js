// Theme: a switch between light and dark. Until Dhira flips it, the page
// follows the system setting (and its changes); a flip is remembered in
// localStorage. Loaded in <head>, so it runs before paint. Without script
// the system setting applies (the CSS media query) and the switch stays hidden.
(function () {
  'use strict';
  var KEY = 'camelot-theme';
  var root = document.documentElement;
  var system = window.matchMedia('(prefers-color-scheme: dark)');
  var syncing = false;

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

  // apply sets the class and puts the switch (the shipped component: its
  // hidden checkbox's change event syncs the visible part) in step.
  function apply() {
    var dark = current() === 'dark';
    root.classList.toggle('dark', dark);
    root.classList.toggle('light', !dark);
    var wrap = document.querySelector('[data-theme-switch]');
    var input = document.getElementById('theme-switch');
    if (!wrap || !input) return;
    wrap.hidden = false;
    if (input.checked !== dark) {
      input.checked = dark;
      syncing = true;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      syncing = false;
    }
  }

  apply();
  document.addEventListener('DOMContentLoaded', apply);
  system.addEventListener('change', apply);
  document.addEventListener('change', function (e) {
    if (syncing || e.target.id !== 'theme-switch') return;
    try { localStorage.setItem(KEY, e.target.checked ? 'dark' : 'light'); } catch (err) { /* private mode: this page only */ }
    apply();
  });
})();
