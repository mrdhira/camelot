// Side panel: a click on a card link marked data-panel (no modifier keys)
// opens the card in the panel instead of leaving the page; the link keeps
// working as a page link everywhere else. The panel is loaded from
// /tasks/{id}/panel.
(function () {
  'use strict';
  var body = document.querySelector('[data-panel-body]');
  if (!body) return;
  var seq = 0;

  document.addEventListener('click', function (e) {
    var link = e.target.closest && e.target.closest('a[data-panel]');
    if (!link || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    if (!(window.templ && window.templ.dialog)) return;
    e.preventDefault();
    open(link.dataset.panel, link);
  });

  function open(id, trigger) {
    var mine = ++seq;
    body.innerHTML = '<p class="text-sm text-muted-foreground">Loading ' + escapeText(id) + '…</p>';
    window.templ.dialog.open('panel', trigger);
    fetch('/tasks/' + encodeURIComponent(id) + '/panel', { credentials: 'same-origin' })
      .then(function (r) { return r.ok ? r.text() : Promise.reject(r.status); })
      .then(function (html) {
        if (mine !== seq) return;
        body.innerHTML = html;
        var title = body.querySelector('[data-panel-title]');
        if (title) { title.setAttribute('tabindex', '-1'); title.focus(); }
      })
      .catch(function () {
        if (mine !== seq) return;
        body.innerHTML = '<p class="text-sm">The card could not be loaded. <a class="underline" href="/tasks/' + encodeURIComponent(id) + '">Open its page</a>.</p>';
      });
  }

  function escapeText(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }
})();
