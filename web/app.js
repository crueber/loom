'use strict';
/* Loom rebuild frontend (vanilla JS, no framework — initial bundle budget).
 * Instant-load: paint from localStorage cache synchronously, then
 * revalidate from network in the background. Server bootstrap
 * (window.__BOOTSTRAP_DATA__) seeds the cache on cold starts.
 * Writes are optimistic-local first, synced behind, server = truth.
 */
(function () {
  var t0 = (window.performance && performance.now()) || 0;
  var LS_KEY = 'loom.cache.v1';
  var boardEl = document.getElementById('board');
  var boardsEl = document.getElementById('boards');
  var state = { boards: [], tree: null, boardId: null };

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  // Tiny markdown: **bold**, *italic*, `code`, [text](url), lines. No deps.
  function md(s) {
    var h = esc(s);
    h = h.replace(/`([^`]+)`/g, '<code>$1</code>');
    h = h.replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    h = h.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    h = h.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    return h.replace(/\n/g, '<br>');
  }
  function favicon(url) {
    try { return 'https://www.google.com/s2/favicons?domain=' + new URL(url).hostname + '&sz=64'; }
    catch (e) { return ''; }
  }
  function host(url) { try { return new URL(url).hostname; } catch (e) { return url; } }

  function loadCache() {
    try {
      var raw = localStorage.getItem(LS_KEY);
      if (raw) return JSON.parse(raw);
    } catch (e) {}
    return null;
  }
  function saveCache() {
    try { localStorage.setItem(LS_KEY, JSON.stringify({ boards: state.boards, tree: state.tree })); }
    catch (e) {} // quota: never block paint on cache writes
  }

  function api(method, path, body) {
    return fetch(path, {
      method: method,
      headers: { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    }).then(function (r) {
      if (!r.ok) throw new Error('HTTP ' + r.status + ' ' + path);
      if (r.status === 204) return null;
      return r.json();
    });
  }

  function blockHTML(b) {
    if (b.type === 'link' && b.url) {
      var fav = favicon(b.url);
      return '<div class="block"><a class="link" href="' + esc(b.url) + '" target="_blank" rel="noopener">' +
        (fav ? '<img src="' + fav + '" alt="" loading="lazy" width="22" height="22">' : '') +
        '<span><span class="t">' + esc(b.title || b.url) + '</span><br><span class="u">' + esc(host(b.url)) + '</span></span></a></div>';
    }
    if (b.type === 'image' && (b.thumb_url || b.image_url)) {
      var src = b.thumb_url || b.image_url;
      var full = b.image_url || b.thumb_url;
      return '<div class="block"><a href="' + esc(full) + '" target="_blank" rel="noopener">' +
        '<img class="photo" src="' + esc(src) + '" alt="' + esc(b.alt || '') + '" loading="lazy" decoding="async"></a></div>';
    }
    if (b.content) return '<div class="block note" data-block="' + b.id + '">' + md(b.content) + '</div>';
    return '';
  }

  function cardHTML(card) {
    var inner = card.blocks.map(blockHTML).join('') || '<div class="block note" style="color:var(--muted)">Empty card — click edit to add text.</div>';
    return '<article class="card" data-card="' + card.id + '">' + inner +
      '<div class="cardbar"><button data-act="edit">edit</button>' +
      '<button data-act="addlink">+ link</button><button data-act="addnote">+ note</button>' +
      '<button data-act="addimg">+ image</button><button data-act="addimgurl">+ img url</button><button data-act="del">delete</button></div></article>';
  }

  function render() {
    // Board switcher.
    boardsEl.innerHTML = state.boards.map(function (b) {
      return '<button data-board="' + b.id + '"' + (b.id === state.boardId ? ' class="active"' : '') + '>' + esc(b.title) + '</button>';
    }).join('');
    if (!state.tree) { boardEl.innerHTML = ''; return; }
    boardEl.innerHTML = state.tree.columns.map(function (col) {
      var cards = (state.tree.cards[col.id] || []).map(cardHTML).join('');
      return '<section class="column' + (col.collapsed ? ' collapsed' : '') + '" data-col="' + col.id + '">' +
        '<h2><button class="fold" data-fold="' + col.id + '" title="Collapse/expand">' + (col.collapsed ? '▸' : '▾') + '</button>' +
        '<span class="dot" style="background:' + esc(col.color || '#ccc') + '"></span>' +
        '<span class="editable" data-edit-col="' + col.id + '">' + esc(col.title) + '</span></h2>' +
        '<div class="cards">' + cards + '<button class="add-card" data-add="' + col.id + '">+ Add card</button></div></section>';
    }).join('');
  }

  function findCard(id) {
    var cols = state.tree ? state.tree.columns : [];
    for (var i = 0; i < cols.length; i++) {
      var cards = state.tree.cards[cols[i].id] || [];
      for (var j = 0; j < cards.length; j++) if (cards[j].id === id) return cards[j];
    }
    return null;
  }

  // Optimistic PATCH with background sync; server is source of truth.
  function syncCard(card) {
    saveCache();
    api('PATCH', '/api/cards/' + card.id, card).then(function (fresh) {
      Object.assign(card, fresh);
      saveCache(); render();
    }).catch(function () { /* stays local; revalidates next load */ });
  }

  boardEl.addEventListener('click', function (e) {
    var t = e.target;
    if (t.dataset && t.dataset.fold) {
      // Collapse toggle: optimistic-local, synced behind.
      var colId = t.dataset.fold;
      var target = null;
      (state.tree.columns || []).forEach(function (c) { if (c.id === colId) target = c; });
      if (!target) return;
      target.collapsed = !target.collapsed;
      saveCache(); render();
      api('PATCH', '/api/columns/' + colId, target).catch(function () {});
      return;
    }
    if (t.dataset && t.dataset.add) {
      var colId = t.dataset.add;
      api('POST', '/api/columns/' + colId + '/cards', { blocks: [{ type: 'note', content: 'New note' }] })
        .then(function (card) {
          state.tree.cards[colId] = (state.tree.cards[colId] || []).concat([card]);
          saveCache(); render();
        });
      return;
    }
    var bar = t.closest && t.closest('[data-act]');
    if (!bar) return;
    var cardEl = t.closest('[data-card]');
    var card = cardEl && findCard(cardEl.dataset.card);
    if (!card) return;
    var act = bar.dataset.act;
    if (act === 'del') {
      cardEl.remove();
      api('DELETE', '/api/cards/' + card.id).then(revalidate).catch(function () {});
      // drop locally
      Object.keys(state.tree.cards).forEach(function (k) {
        state.tree.cards[k] = state.tree.cards[k].filter(function (c) { return c.id !== card.id; });
      });
      saveCache(); render();
    } else if (act === 'addlink') {
      var url = prompt('Link URL:');
      if (!url) return;
      card.blocks.push({ id: 'tmp-' + Date.now(), type: 'link', url: url, title: url, position: card.blocks.length });
      syncCard(card); render();
    } else if (act === 'addnote') {
      card.blocks.push({ id: 'tmp-' + Date.now(), type: 'note', content: 'New note', position: card.blocks.length });
      syncCard(card); render();
    } else if (act === 'addimg') {
      // Local upload: hidden file picker -> POST /api/images -> image block.
      var input = document.createElement('input');
      input.type = 'file';
      input.accept = 'image/jpeg,image/png,image/gif';
      input.onchange = function () {
        if (!input.files.length) return;
        var fd = new FormData();
        fd.append('file', input.files[0]);
        fetch('/api/images', { method: 'POST', body: fd }).then(function (r) {
          if (!r.ok) throw new Error('upload failed');
          return r.json();
        }).then(function (up) {
          card.blocks.push({ id: 'tmp-' + Date.now(), type: 'image', image_url: up.url, thumb_url: up.thumb_url, position: card.blocks.length });
          syncCard(card); render();
        }).catch(function () { alert('Image upload failed (jpeg/png/gif, max 12MB).'); });
      };
      input.click();
    } else if (act === 'addimgurl') {
      var src = prompt('Image URL:');
      if (!src) return;
      card.blocks.push({ id: 'tmp-' + Date.now(), type: 'image', image_url: src, position: card.blocks.length });
      syncCard(card); render();
    } else if (act === 'edit') {
      // Inline editing: notes become contenteditable in place (no flip panels).
      var notes = cardEl.querySelectorAll('.note');
      if (!notes.length) {
        card.blocks.push({ id: 'tmp-' + Date.now(), type: 'note', content: 'New note', position: card.blocks.length });
        syncCard(card); render(); return;
      }
      notes.forEach(function (n) {
        n.contentEditable = 'true';
        n.focus();
        n.addEventListener('blur', function handler() {
          n.removeEventListener('blur', handler);
          n.contentEditable = 'false';
          var b = null;
          card.blocks.forEach(function (x) { if (x.id === n.dataset.block) b = x; });
          var text = n.innerText;
          if (b && b.content !== text) { b.content = text; syncCard(card); }
          render();
        });
      });
    }
  });

  boardsEl.addEventListener('click', function (e) {
    var b = e.target.closest && e.target.closest('[data-board]');
    if (b) selectBoard(b.dataset.board);
  });
  document.getElementById('add-board').addEventListener('click', function () {
    var title = prompt('Board name:');
    if (!title) return;
    api('POST', '/api/boards', { title: title }).then(function (nb) {
      state.boards.push(nb); selectBoard(nb.id);
    });
  });
  document.getElementById('add-column').addEventListener('click', function () {
    if (!state.boardId) return;
    var title = prompt('Column name:');
    if (!title) return;
    api('POST', '/api/boards/' + state.boardId + '/columns', { title: title, color: '#4c8dff' }).then(revalidate);
  });

  function selectBoard(id) {
    state.boardId = id;
    api('GET', '/api/boards/' + id).then(function (tree) {
      state.tree = tree; saveCache(); render();
    }).catch(function () {
      var cached = loadCache();
      if (cached && cached.tree && cached.tree.board.id === id) { state.tree = cached.tree; render(); }
    });
  }

  function revalidate() {
    api('GET', '/api/boards').then(function (boards) {
      state.boards = boards;
      if (!state.boardId && boards.length) state.boardId = boards[0].id;
      if (!state.boardId) { saveCache(); render(); return; }
      return api('GET', '/api/boards/' + state.boardId).then(function (tree) {
        state.tree = tree; saveCache(); render();
      });
    }).catch(function () {});
  }

  // ---- Instant-load boot: cache-first synchronous paint ----
  var boot = window.__BOOTSTRAP_DATA__;
  var cached = loadCache();
  if (cached && cached.tree) {
    state.boards = cached.boards || [];
    state.tree = cached.tree;
    state.boardId = cached.tree.board.id;
  } else if (boot && boot.board) {
    state.boards = [{ id: boot.board.id, title: boot.board.title }];
    state.tree = boot;
    state.boardId = boot.board.id;
  }
  render();
  if (window.performance && performance.now) {
    window.__LOOM_FIRST_PAINT_MS = Math.round((performance.now() - t0) * 10) / 10;
  }
  // Background revalidation never blocks first paint.
  if (window.requestIdleCallback) requestIdleCallback(revalidate);
  else setTimeout(revalidate, 0);
})();
