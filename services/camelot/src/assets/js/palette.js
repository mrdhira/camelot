// ⌘K / Ctrl+K palette: searches cards, documents and pages through /search;
// arrow keys choose, Enter opens. "/" opens it too when no field has focus.
// The palette is reset when it closes, never when it opens: the dialog
// focuses the box itself, and a reset after typing began would wipe it.
(function () {
  'use strict';
  var form = document.querySelector('[data-palette-form]');
  if (!form) return;
  var input = form.querySelector('input[type=search]');
  var list = form.querySelector('[data-palette-results]');
  var status = form.querySelector('[data-palette-status]');
  var results = [];
  var shownFor = null;
  var active = -1;
  var timer = null;
  var seq = 0;

  function dialog() { return window.templ && window.templ.dialog; }

  document.addEventListener('keydown', function (e) {
    var typing = /^(INPUT|TEXTAREA|SELECT)$/.test((document.activeElement || {}).tagName || '');
    if ((e.key === 'k' || e.key === 'K') && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      openPalette();
    } else if (e.key === '/' && !typing && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      openPalette();
    }
  });

  document.addEventListener('dialog-close', function (e) {
    if (e.target && e.target.id === 'palette') reset();
  });

  function openPalette() {
    var d = dialog();
    if (!d) return;
    if (shownFor === null) search(input.value);
    d.open('palette');
  }

  function reset() {
    clearTimeout(timer);
    seq++;
    input.value = '';
    results = [];
    shownFor = null;
    active = -1;
    paint();
    status.textContent = '';
  }

  input.addEventListener('focus', function () {
    if (shownFor === null) search(input.value);
  });

  input.addEventListener('input', function () {
    clearTimeout(timer);
    timer = setTimeout(function () { search(input.value); }, 120);
  });

  input.addEventListener('keydown', function (e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (!results.length) return;
      active = (active + (e.key === 'ArrowDown' ? 1 : -1) + results.length) % results.length;
      paint();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (shownFor === input.value) {
        go();
      } else {
        clearTimeout(timer);
        search(input.value).then(function (ok) { if (ok) go(); });
      }
    }
  });

  function go() {
    if (active >= 0 && results[active]) location.href = results[active].href;
  }

  // search shows the results for q; it resolves true when they are shown.
  function search(q) {
    var mine = ++seq;
    return fetch('/search?q=' + encodeURIComponent(q), { credentials: 'same-origin' })
      .then(function (r) { return r.ok ? r.json() : Promise.reject(r.status); })
      .then(function (data) {
        if (mine !== seq) return false;
        results = data || [];
        shownFor = q;
        active = results.length ? 0 : -1;
        paint();
        status.textContent = results.length ? results.length + ' results' : 'Nothing matches';
        return true;
      })
      .catch(function () {
        if (mine === seq) status.textContent = 'Search failed';
        return false;
      });
  }

  function paint() {
    list.textContent = '';
    results.forEach(function (r, i) {
      var li = document.createElement('li');
      li.id = 'palette-opt-' + i;
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', i === active ? 'true' : 'false');
      li.className = 'rounded-md' + (i === active ? ' bg-muted' : '');
      var a = document.createElement('a');
      a.href = r.href;
      a.tabIndex = -1;
      a.className = 'grid gap-0.5 px-2.5 py-1.5 text-sm';
      var top = document.createElement('span');
      top.className = 'flex items-baseline gap-2';
      var kind = document.createElement('span');
      kind.className = 'w-10 shrink-0 text-[11px] uppercase text-muted-foreground';
      kind.textContent = r.kind;
      var title = document.createElement('span');
      title.className = 'font-medium';
      title.textContent = (r.kind === 'page' ? '' : r.id + ' · ') + r.title;
      top.append(kind, title);
      var detail = document.createElement('span');
      detail.className = 'pl-12 text-xs text-muted-foreground';
      detail.textContent = r.detail;
      a.append(top, detail);
      li.append(a);
      li.addEventListener('mousemove', function () { if (active !== i) { active = i; paint(); } });
      list.append(li);
    });
    input.setAttribute('aria-activedescendant', active >= 0 ? 'palette-opt-' + active : '');
    var el = document.getElementById('palette-opt-' + active);
    if (el) el.scrollIntoView({ block: 'nearest' });
  }
})();
