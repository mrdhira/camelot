// Board drag and drop, by delegation so it survives live refreshes:
// dropping a card on another column posts the board's move form for that
// card and stage. Without script, a card's page (or panel) moves it.
(function () {
  'use strict';
  var dragged = null;

  function setDraggable() {
    document.querySelectorAll('[data-card]').forEach(function (c) { c.setAttribute('draggable', 'true'); });
  }
  setDraggable();
  document.addEventListener('camelot:refreshed', setDraggable);

  document.addEventListener('dragstart', function (e) {
    var card = e.target.closest && e.target.closest('[data-card]');
    if (!card) return;
    dragged = card;
    card.classList.add('opacity-50');
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', card.dataset.card);
  });

  document.addEventListener('dragend', function () {
    if (dragged) dragged.classList.remove('opacity-50');
    dragged = null;
    document.querySelectorAll('[data-column]').forEach(function (c) { c.classList.remove('ring-2'); });
  });

  document.addEventListener('dragover', function (e) {
    var column = dragged && e.target.closest && e.target.closest('[data-column]');
    if (!column) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    document.querySelectorAll('[data-column]').forEach(function (c) { c.classList.toggle('ring-2', c === column); });
  });

  document.addEventListener('drop', function (e) {
    var column = dragged && e.target.closest && e.target.closest('[data-column]');
    if (!column) return;
    e.preventDefault();
    column.classList.remove('ring-2');
    if (dragged.dataset.status === column.dataset.column) return;
    var form = document.getElementById('board-move');
    form.action = '/tasks/' + encodeURIComponent(dragged.dataset.card) + '/move';
    form.elements.status.value = column.dataset.column;
    form.elements.updated_at.value = dragged.dataset.updated;
    form.submit();
  });
})();
