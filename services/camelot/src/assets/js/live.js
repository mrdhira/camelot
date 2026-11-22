// Live: every 10 s, asks /changes/stamp whether any card or document
// changed; if so, swaps in the page's fresh content in place and marks the
// cards that changed. Never while a field has focus, a dialog is open or a
// card is being dragged. Keeps "updated N s ago" in the header current.
(function () {
  'use strict';
  var INTERVAL = 10000;
  var live = document.getElementById('live');
  var text = live && live.querySelector('[data-live-text]');
  var asOf = live ? new Date(live.dataset.asOf) : new Date();
  var stamp = document.body.dataset.stamp || '';

  // busy is true while a refresh would lose something: focus or an
  // unsaved value in a field, a details block the user opened or closed,
  // a drag, or an open dialog.
  function busy() {
    var a = document.activeElement;
    if (a && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName)) return true;
    if (document.querySelector('[data-card].opacity-50, #main details[data-toggled]')) return true;
    var fields = document.querySelectorAll('#main input, #main textarea, #main select');
    for (var i = 0; i < fields.length; i++) if (dirty(fields[i])) return true;
    var d = window.templ && window.templ.dialog;
    return !!(d && (d.isOpen('palette') || d.isOpen('panel')));
  }

  function dirty(f) {
    if (f.type === 'hidden' || f.type === 'submit') return false;
    if (f.type === 'checkbox' || f.type === 'radio') return f.checked !== f.defaultChecked;
    if (f.tagName === 'SELECT') {
      for (var i = 0; i < f.options.length; i++) if (f.options[i].selected !== f.options[i].defaultSelected) return true;
      return false;
    }
    return f.value !== f.defaultValue;
  }

  document.addEventListener('toggle', function (e) {
    if (e.target.tagName === 'DETAILS') e.target.setAttribute('data-toggled', '');
  }, true);

  function ago() {
    if (!text) return;
    var s = Math.max(0, Math.round((Date.now() - asOf.getTime()) / 1000));
    text.textContent = s < 5 ? 'updated just now' : s < 90 ? 'updated ' + s + ' s ago' : 'updated ' + Math.round(s / 60) + ' min ago';
  }

  function versions(root) {
    var out = {};
    root.querySelectorAll('[data-card]').forEach(function (el) { out[el.dataset.card] = el.dataset.updated; });
    return out;
  }

  function refresh() {
    var before = versions(document);
    return fetch(location.href, { headers: { 'X-Camelot-Live': '1' }, credentials: 'same-origin' })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (html) {
        if (busy()) return;
        var next = new DOMParser().parseFromString(html, 'text/html');
        var main = next.getElementById('main');
        var nav = next.querySelector('[data-nav]');
        if (!main) return;
        var y = window.scrollY;
        document.getElementById('main').replaceWith(main);
        if (nav) document.querySelector('[data-nav]').replaceWith(nav);
        window.scrollTo(0, y);
        stamp = next.body.dataset.stamp || stamp;
        document.body.dataset.stamp = stamp;
        asOf = new Date();
        main.querySelectorAll('[data-card]').forEach(function (el) {
          if (before[el.dataset.card] !== el.dataset.updated) el.classList.add('is-fresh');
        });
        document.dispatchEvent(new CustomEvent('camelot:refreshed'));
      })
      .catch(function () {});
  }

  function poll() {
    if (document.hidden) return;
    fetch('/changes/stamp', { credentials: 'same-origin', cache: 'no-store' })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (now) {
        if (now === stamp) { asOf = new Date(); return; }
        if (busy()) return;
        return refresh();
      })
      .catch(function () {});
  }

  setInterval(poll, INTERVAL);
  setInterval(ago, 1000);
  document.addEventListener('visibilitychange', function () { if (!document.hidden) poll(); });
  ago();
})();
