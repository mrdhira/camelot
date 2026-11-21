// Board drag and drop: dropping a card on a column posts the board's move
// form for that card and status. Without script, a card's page moves it.
(function () {
  'use strict';
  var form = document.getElementById('board-move');
  if (!form) return;
  var dragged = null;

  document.querySelectorAll('[data-card]').forEach(function (card) {
    card.setAttribute('draggable', 'true');
    card.addEventListener('dragstart', function (e) {
      dragged = card;
      card.classList.add('opacity-50');
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', card.dataset.card);
    });
    card.addEventListener('dragend', function () {
      card.classList.remove('opacity-50');
      dragged = null;
      document.querySelectorAll('[data-column]').forEach(function (c) { c.classList.remove('ring-2'); });
    });
  });

  document.querySelectorAll('[data-column]').forEach(function (column) {
    column.addEventListener('dragover', function (e) {
      if (!dragged) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      column.classList.add('ring-2');
    });
    column.addEventListener('dragleave', function (e) {
      if (!column.contains(e.relatedTarget)) column.classList.remove('ring-2');
    });
    column.addEventListener('drop', function (e) {
      e.preventDefault();
      column.classList.remove('ring-2');
      if (!dragged || dragged.dataset.status === column.dataset.column) return;
      form.action = '/tasks/' + encodeURIComponent(dragged.dataset.card) + '/move';
      form.elements.status.value = column.dataset.column;
      form.elements.updated_at.value = dragged.dataset.updated;
      form.submit();
    });
  });
})();
