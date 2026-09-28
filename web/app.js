'use strict';
/* Loom rebuild frontend (vanilla JS, no framework — initial bundle budget).
 * Instant-load: paint from localStorage cache synchronously, then
 * revalidate from network in the background. Server bootstrap
 * (window.__BOOTSTRAP_DATA__) seeds the cache on cold starts.
 * Writes are optimistic-local first, synced behind, server = truth.
 *
 * UX model (Columns.app feel): everything edits inline — click board
 * title, column titles, or any note to edit in place. New notes focus
 * immediately. Column dots recolor, swatches theme the board, collapsing
 * columns folds them into slim rails (one open column = reading mode).
 */
(function () {
  var t0 = (window.performance && performance.now()) || 0;
  var LS_KEY = 'loom.cache.v2';
  var LS_OLD = 'loom.cache.v1';
  var boardEl = document.getElementById('board');
  var boardsEl = document.getElementById('boards');
  var creatorEl = document.getElementById('creator');
  var state = { boards: [], trees: {}, boardId: null };
  var BG_DEFAULT = '';

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

  function normalizeTree(tree) {
    if (!tree) return null;
    if (!tree.columns) tree.columns = [];
    if (!tree.cards) tree.cards = {};
    tree.columns.forEach(function (c) {
      if (!tree.cards[c.id]) tree.cards[c.id] = [];
    });
    return tree;
  }

  function loadCache() {
    try {
      var raw = localStorage.getItem(LS_KEY);
      if (raw) {
        var c = JSON.parse(raw);
        if (c && c.trees) {
          Object.keys(c.trees).forEach(function (k) { c.trees[k] = normalizeTree(c.trees[k]); });
          return c;
        }
      }
      // Migrate v1 single-tree cache.
      var old = localStorage.getItem(LS_OLD);
      if (old) {
        var v1 = JSON.parse(old);
        if (v1 && v1.tree) {
          var t = normalizeTree(v1.tree);
          var trees = {};
          trees[t.board.id] = t;
          return { boards: v1.boards || [], trees: trees, boardId: t.board.id };
        }
      }
    } catch (e) {}
    return null;
  }
  function saveCache() {
    try { localStorage.setItem(LS_KEY, JSON.stringify({ boards: state.boards, trees: state.trees, boardId: state.boardId })); }
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

  function curTree() { return state.boardId ? state.trees[state.boardId] : null; }

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
    if (b.type === 'note' || b.content) {
      var empty = !b.content;
      return '<div class="block note' + (empty ? ' empty' : '') + '" data-block="' + esc(b.id || '') + '"' +
        ' title="Click to edit">' + (empty ? 'Write something&hellip;' : md(b.content)) + '</div>';
    }
    return '';
  }

  function cardHTML(card) {
    var inner = card.blocks.map(blockHTML).join('') ||
      '<div class="block note empty" data-block="" title="Click to edit">Write something&hellip;</div>';
    return '<article class="card" data-card="' + card.id + '">' + inner +
      '<div class="cardbar"><button data-act="edit">edit</button>' +
      '<button data-act="addlink">+ link</button><button data-act="addnote">+ note</button>' +
      '<button data-act="addimg">+ image</button><button data-act="addimgurl">+ img url</button><button data-act="del">delete</button></div></article>';
  }

  var SWATCHES = [
    { id: '', name: 'Paper' },
    { id: 'honey', name: 'Honey' },
    { id: 'sage', name: 'Sage' },
    { id: 'sky', name: 'Sky' },
    { id: 'rose', name: 'Rose' },
    { id: 'slate', name: 'Ink' },
  ];

  function render() {
    // Board switcher.
    boardsEl.innerHTML = state.boards.map(function (b) {
      return '<button data-board="' + b.id + '"' + (b.id === state.boardId ? ' class="active"' : '') + '>' + esc(b.title) + '</button>';
    }).join('');
    var tree = curTree();
    if (!tree) {
      document.body.dataset.bg = BG_DEFAULT;
      boardEl.innerHTML = state.boards.length
        ? '<div class="emptyboard"><p>Select a board above, or create one to start writing.</p></div>'
        : '<div class="emptyboard"><p>Welcome to Loom — create your first board to start writing.</p><button class="primary" data-newboard>＋ New board</button></div>';
      return;
    }
    var bg = tree.board.background || BG_DEFAULT;
    document.body.dataset.bg = bg;
    var cols = tree.columns || [];
    var open = cols.filter(function (c) { return !c.collapsed; });
    var focus = open.length === 1 && cols.length > 1;
    var allFolded = cols.length > 0 && open.length === 0;
    var head = '<div class="boardhead"><h1 class="boardtitle" id="boardtitle" title="Click to rename">' +
      esc(tree.board.title) + '</h1><div class="swatches" role="group" aria-label="Board background">' +
      SWATCHES.map(function (s) {
        return '<button data-bg="' + s.id + '"' + ((bg || '') === s.id ? ' class="on"' : '') +
          ' title="' + s.name + '" aria-label="' + s.name + ' background"><i class="sw sw-' + (s.id || 'paper') + '"></i></button>';
      }).join('') + '</div>' +
      (cols.length ? '<button class="ghost" data-foldall>' + (allFolded ? 'Expand all' : 'Collapse all') + '</button>' : '') +
      '</div>';
    if (!cols.length) {
      boardEl.innerHTML = head + '<div class="emptyboard"><p>This board has no columns yet.</p><button class="primary" data-newcol>Add your first column</button></div>';
      return;
    }
    boardEl.innerHTML = head + '<div class="cols' + (focus ? ' focus' : '') + '">' + cols.map(function (col) {
      var cards = (tree.cards[col.id] || []).map(cardHTML).join('');
      return '<section class="column' + (col.collapsed ? ' collapsed' : '') + (focus && !col.collapsed ? ' reading' : '') + '" data-col="' + col.id + '">' +
        '<h2><button class="fold" data-fold="' + col.id + '" title="' + (col.collapsed ? 'Expand' : 'Collapse') + '">' + (col.collapsed ? '▸' : '▾') + '</button>' +
        '<label class="cdot" style="background:' + esc(col.color || '#c9c4b6') + '" title="Column color">' +
        '<input type="color" data-colcolor="' + col.id + '" value="' + esc(col.color || '#4c8dff') + '" tabindex="-1"></label>' +
        '<span class="coltitle" data-coltitle="' + col.id + '" title="Click to rename">' + esc(col.title) + '</span>' +
        '<span class="colcount">' + (tree.cards[col.id] || []).length + '</span></h2>' +
        '<div class="cards">' + cards + '<button class="add-card" data-add="' + col.id + '">+ Add card</button></div></section>';
    }).join('') + '</div>';
  }

  function findCard(id) {
    var tree = curTree();
    if (!tree) return null;
    var cols = tree.columns || [];
    for (var i = 0; i < cols.length; i++) {
      var cards = tree.cards[cols[i].id] || [];
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

  function startEdit(el, selectAll) {
    if (!el || el.isContentEditable) return;
    el.contentEditable = 'true';
    el.classList.add('editing');
    el.focus();
    try {
      var range = document.createRange();
      range.selectNodeContents(el);
      if (!selectAll) range.collapse(false);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    } catch (e) {}
  }

  function commitNote(el) {
    var cardEl = el.closest && el.closest('[data-card]');
    if (!cardEl) return;
    var card = findCard(cardEl.dataset.card);
    if (!card) return;
    var text = el.innerText.replace(/\n+$/, '');
    var blk = null;
    card.blocks.forEach(function (x) { if (x.id === (el.dataset.block || '') || (el.dataset.block === '' && !x.id)) blk = x; });
    // Empty-id blocks (just created): match the last empty-content note.
    if (!blk && el.dataset.block === '') {
      for (var i = card.blocks.length - 1; i >= 0; i--) {
        if (card.blocks[i].type === 'note' && !card.blocks[i].content) { blk = card.blocks[i]; break; }
      }
    }
    if (blk && (blk.content || '') !== text) {
      blk.content = text;
      if (blk.type !== 'note' && text) blk.type = 'note';
      syncCard(card);
    }
    render();
  }

  // Focus a note right after its render commit. Runs synchronously:
  // render() builds DOM in the same task, so the node is queryable
  // immediately (rAF focus is unreliable in background/headless tabs).
  function focusNote(cardId, blockId, selectAll) {
    var cardEl = boardEl.querySelector('[data-card="' + cardId + '"]');
    if (!cardEl) return;
    var target = blockId ? cardEl.querySelector('[data-block="' + blockId + '"]') : null;
    if (!target) {
      var notes = cardEl.querySelectorAll('.note');
      target = notes.length ? notes[notes.length - 1] : null;
    }
    if (target) startEdit(target, selectAll !== false);
    else cardEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }

  // Inline single-line composer inside a card (replaces prompt()).
  function askInCard(cardEl, label, placeholder) {
    closeCreator();
    return new Promise(function (resolve) {
      var box = document.createElement('div');
      box.className = 'composer';
      box.innerHTML = '<input type="text" placeholder="' + esc(placeholder || label) + '" aria-label="' + esc(label) + '">' +
        '<button data-ok>Add</button><button data-cancel aria-label="Cancel">✕</button>';
      cardEl.appendChild(box);
      var input = box.querySelector('input');
      input.focus();
      function done(val) { box.remove(); resolve(val); }
      box.querySelector('[data-ok]').addEventListener('click', function () { done(input.value.trim()); });
      box.querySelector('[data-cancel]').addEventListener('click', function () { done(null); });
      input.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') done(input.value.trim());
        else if (e.key === 'Escape') done(null);
      });
    });
  }

  // Header inline creator form (replaces prompt() for boards/columns).
  function openCreator(kind) {
    closeCreator();
    var box = document.createElement('div');
    box.className = 'creatorform';
    box.innerHTML = '<input type="text" placeholder="' + (kind === 'board' ? 'Board name…' : 'Column name…') + '" aria-label="' +
      (kind === 'board' ? 'New board name' : 'New column name') + '">' +
      '<button data-ok>Create</button><button data-cancel aria-label="Cancel">✕</button>';
    creatorEl.appendChild(box);
    var input = box.querySelector('input');
    input.focus();
    function done(commit) {
      var val = input.value.trim();
      box.remove();
      if (!commit || !val) return;
      if (kind === 'board') {
        api('POST', '/api/boards', { title: val }).then(function (nb) {
          state.boards.push(nb);
          state.trees[nb.id] = normalizeTree({ board: nb, columns: [], cards: {} });
          selectBoard(nb.id);
        });
      } else if (state.boardId) {
        api('POST', '/api/boards/' + state.boardId + '/columns', { title: val, color: '#4c8dff' }).then(revalidate);
      }
    }
    box.querySelector('[data-ok]').addEventListener('click', function () { done(true); });
    box.querySelector('[data-cancel]').addEventListener('click', function () { done(false); });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') done(true);
      else if (e.key === 'Escape') done(false);
    });
  }
  function closeCreator() { creatorEl.innerHTML = ''; }

  boardEl.addEventListener('click', function (e) {
    var t = e.target;
    if (t.closest && t.closest('[data-newboard]')) { openCreator('board'); return; }
    if (t.closest && t.closest('[data-newcol]')) { openCreator('column'); return; }
    // Board background swatches (scoped: body also carries data-bg).
    var sw = t.closest && t.closest('.swatches [data-bg]');
    if (sw) {
      var tree = curTree();
      if (!tree) return;
      tree.board.background = sw.dataset.bg;
      saveCache(); render();
      api('PATCH', '/api/boards/' + tree.board.id, { background: sw.dataset.bg }).then(function (b) {
        tree.board = b; saveCache(); render();
      }).catch(function () {});
      return;
    }
    // Collapse all / expand all.
    if (t.closest && t.closest('[data-foldall]')) {
      var tr = curTree();
      if (!tr) return;
      var anyOpen = (tr.columns || []).some(function (c) { return !c.collapsed; });
      (tr.columns || []).forEach(function (c) {
        c.collapsed = anyOpen;
        api('PATCH', '/api/columns/' + c.id, c).catch(function () {});
      });
      saveCache(); render();
      return;
    }
    // Board title rename.
    if (t.closest && t.closest('#boardtitle')) {
      var bt = document.getElementById('boardtitle');
      startEdit(bt, true);
      return;
    }
    // Column title rename.
    var ct = t.closest && t.closest('[data-coltitle]');
    if (ct) { startEdit(ct, true); return; }
    if (t.dataset && t.dataset.fold) {
      // Collapse toggle: optimistic-local, synced behind.
      var colId = t.dataset.fold;
      var target = null;
      var cols = (curTree() && curTree().columns) || [];
      cols.forEach(function (c) { if (c.id === colId) target = c; });
      if (!target) return;
      target.collapsed = !target.collapsed;
      saveCache(); render();
      api('PATCH', '/api/columns/' + colId, target).catch(function () {});
      return;
    }
    if (t.dataset && t.dataset.add) {
      var addCol = t.dataset.add;
      api('POST', '/api/columns/' + addCol + '/cards', { blocks: [{ type: 'note', content: '' }] })
        .then(function (card) {
          var tree2 = curTree();
          if (!tree2) return;
          tree2.cards[addCol] = (tree2.cards[addCol] || []).concat([card]);
          saveCache(); render();
          focusNote(card.id, (card.blocks[0] && card.blocks[0].id) || '', true);
        });
      return;
    }
    // Click any note text to edit it directly (links still navigate).
    var note = t.closest && t.closest('.note[data-block]');
    if (note && !note.isContentEditable && !(t.tagName === 'A')) {
      // Empty-card placeholder also lands here via .note.empty[data-block=""].
      var cardEl0 = note.closest('[data-card]');
      var card0 = cardEl0 && findCard(cardEl0.dataset.card);
      if (card0) {
        if (note.dataset.block === '' && !card0.blocks.length) {
          card0.blocks.push({ id: '', type: 'note', content: '', position: 0 });
          saveCache(); render();
          focusNote(card0.id, '', true);
        } else {
          startEdit(note, false);
        }
      }
      return;
    }
    var bar = t.closest && t.closest('[data-act]');
    if (!bar) return;
    var cardEl = t.closest('[data-card]');
    var card = cardEl && findCard(cardEl.dataset.card);
    if (!card) return;
    var act = bar.dataset.act;
    if (act === 'del') {
      Object.keys(curTree().cards).forEach(function (k) {
        curTree().cards[k] = curTree().cards[k].filter(function (c) { return c.id !== card.id; });
      });
      saveCache(); render();
      api('DELETE', '/api/cards/' + card.id).then(revalidate).catch(function () {});
    } else if (act === 'addlink') {
      askInCard(cardEl, 'Link URL', 'Paste link URL…').then(function (url) {
        if (!url) return;
        card.blocks.push({ id: '', type: 'link', url: url, title: url, position: card.blocks.length });
        syncCard(card); render();
      });
    } else if (act === 'addnote') {
      // Render + focus first; the blur commit persists (avoids a
      // sync re-render stealing focus before the user types).
      card.blocks.push({ id: '', type: 'note', content: '', position: card.blocks.length });
      saveCache(); render();
      focusNote(card.id, '', true);
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
          card.blocks.push({ id: '', type: 'image', image_url: up.url, thumb_url: up.thumb_url, position: card.blocks.length });
          syncCard(card); render();
        }).catch(function () { alert('Image upload failed (jpeg/png/gif, max 12MB).'); });
      };
      input.click();
    } else if (act === 'addimgurl') {
      askInCard(cardEl, 'Image URL', 'Paste image URL…').then(function (src) {
        if (!src) return;
        card.blocks.push({ id: '', type: 'image', image_url: src, position: card.blocks.length });
        syncCard(card); render();
      });
    } else if (act === 'edit') {
      // Inline editing: focus the first note (new one if empty).
      // Render + focus first; the blur commit persists.
      var first = cardEl.querySelector('.note[data-block]');
      if (!first || !card.blocks.length) {
        card.blocks.push({ id: '', type: 'note', content: '', position: card.blocks.length });
        saveCache(); render();
        focusNote(card.id, '', true);
        return;
      }
      startEdit(first, false);
    }
  });

  // Column color picker (input bubbles as `input`/`change`).
  boardEl.addEventListener('input', function (e) {
    var t = e.target;
    if (t.dataset && t.dataset.colcolor) {
      var cols = (curTree() && curTree().columns) || [];
      cols.forEach(function (c) { if (c.id === t.dataset.colcolor) c.color = t.value; });
      var dot = t.closest && t.closest('.cdot');
      if (dot) dot.style.background = t.value;
    }
  });
  boardEl.addEventListener('change', function (e) {
    var t = e.target;
    if (t.dataset && t.dataset.colcolor) {
      var target = null;
      ((curTree() && curTree().columns) || []).forEach(function (c) { if (c.id === t.dataset.colcolor) target = c; });
      if (!target) return;
      saveCache(); render();
      api('PATCH', '/api/columns/' + target.id, target).catch(function () {});
    }
  });

  // Commit inline edits on focus loss (blur doesn't bubble; focusout does).
  boardEl.addEventListener('focusout', function (e) {
    var t = e.target;
    if (!t || !t.isContentEditable) return;
    t.contentEditable = 'false';
    t.classList.remove('editing');
    try { window.getSelection().removeAllRanges(); } catch (err) {}
    if (t.id === 'boardtitle') {
      var tree = curTree();
      if (!tree) return;
      var title = t.innerText.trim();
      if (!title) { render(); return; }
      if (title === tree.board.title) { render(); return; }
      tree.board.title = title;
      state.boards.forEach(function (b) { if (b.id === tree.board.id) b.title = title; });
      saveCache(); render();
      api('PATCH', '/api/boards/' + tree.board.id, { title: title }).then(function (b) {
        tree.board = b; saveCache(); render();
      }).catch(function () {});
      return;
    }
    if (t.dataset && t.dataset.coltitle) {
      var col = null;
      ((curTree() && curTree().columns) || []).forEach(function (c) { if (c.id === t.dataset.coltitle) col = c; });
      if (!col) return;
      var ct = t.innerText.trim();
      if (!ct || ct === col.title) { render(); return; }
      col.title = ct;
      saveCache(); render();
      api('PATCH', '/api/columns/' + col.id, col).catch(function () {});
      return;
    }
    if (t.classList && t.classList.contains('note')) commitNote(t);
  });
  // Enter commits single-line titles; Escape cancels.
  boardEl.addEventListener('keydown', function (e) {
    var t = e.target;
    if (!t || !t.isContentEditable) return;
    if (e.key === 'Escape') { e.preventDefault(); t.blur(); render(); }
    else if (e.key === 'Enter' && (t.id === 'boardtitle' || (t.dataset && t.dataset.coltitle))) {
      e.preventDefault(); t.blur();
    }
  });

  boardsEl.addEventListener('click', function (e) {
    var b = e.target.closest && e.target.closest('[data-board]');
    if (b) selectBoard(b.dataset.board);
  });
  document.getElementById('add-board').addEventListener('click', function () { openCreator('board'); });
  document.getElementById('add-column').addEventListener('click', function () {
    if (!state.boardId) return;
    openCreator('column');
  });

  function selectBoard(id) {
    if (!id) return;
    state.boardId = id;
    saveCache();
    var cached = state.trees[id];
    if (cached) render(); // instant paint from per-board cache
    api('GET', '/api/boards/' + id).then(function (tree) {
      state.trees[id] = normalizeTree(tree);
      if (state.boardId === id) { saveCache(); render(); }
    }).catch(function () {
      if (!state.trees[id]) {
        var legacy = loadCache();
        if (legacy && legacy.trees && legacy.trees[id]) { state.trees[id] = legacy.trees[id]; render(); }
      }
    });
  }

  function revalidate() {
    api('GET', '/api/boards').then(function (boards) {
      state.boards = boards || [];
      var known = {};
      state.boards.forEach(function (b) { known[b.id] = true; });
      if (!state.boardId || !known[state.boardId]) {
        state.boardId = state.boards.length ? state.boards[0].id : null;
      }
      if (!state.boardId) { saveCache(); render(); return; }
      return api('GET', '/api/boards/' + state.boardId).then(function (tree) {
        state.trees[state.boardId] = normalizeTree(tree);
        saveCache(); render();
      });
    }).catch(function () {});
  }

  // ---- Instant-load boot: cache-first synchronous paint ----
  var boot = window.__BOOTSTRAP_DATA__;
  var cached = loadCache();
  if (cached && (cached.boardId || Object.keys(cached.trees || {}).length)) {
    state.boards = cached.boards || [];
    state.trees = cached.trees || {};
    state.boardId = cached.boardId || Object.keys(state.trees)[0] || null;
  } else if (boot && boot.board) {
    var seed = normalizeTree(boot);
    state.boards = [{ id: seed.board.id, title: seed.board.title }];
    state.trees = {};
    state.trees[seed.board.id] = seed;
    state.boardId = seed.board.id;
  }
  render();
  if (window.performance && performance.now) {
    window.__LOOM_FIRST_PAINT_MS = Math.round((performance.now() - t0) * 10) / 10;
  }
  // Background revalidation never blocks first paint.
  if (window.requestIdleCallback) requestIdleCallback(revalidate);
  else setTimeout(revalidate, 0);
})();
