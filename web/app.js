'use strict';
/* Loom rebuild frontend (vanilla JS, no framework — initial bundle budget).
 * Instant-load: paint from localStorage cache synchronously, then
 * revalidate from network in the background. Server bootstrap
 * (window.__BOOTSTRAP_DATA__) seeds the cache on cold starts.
 * Writes are optimistic-local first, synced behind, server = truth.
 *
 * UX model (Columns.app feel): everything edits inline — click board
 * title, column titles, or any note to edit in place (no Edit buttons).
 * Enter commits an edit; Ctrl/Cmd+Enter inserts a newline. New notes and
 * new column titles focus immediately. Column dots recolor, swatches theme
 * the board, collapsing columns folds them into slim rails (one open
 * column = reading mode). Boards are deep-linkable via #/b/<id>.
 * Column widths persist in localStorage (loom.colwidths.v1) — documented
 * here because no server column-prefs field exists; collapse persists on
 * the server. Cards/columns reorder via HTML5 drag-drop, persisted through
 * the existing move/PATCH paths. Cards render with only a URL (no note
 * required); adding a link to a fresh empty-note card replaces the
 * placeholder so link-only cards stay link-only.
 */
(function () {
  var t0 = (window.performance && performance.now()) || 0;
  var LS_KEY = 'loom.cache.v2';
  var LS_OLD = 'loom.cache.v1';
  var LS_WIDTHS = 'loom.colwidths.v1'; // colId -> px; see header comment.
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
  // Minimal markdown: "# H1", "## H2", "**bold**", "*italic*", "`code`",
  // "[text](url)", "- list". No deps. Headings/lists are line-based;
  // inline marks apply within each line.
  function inlineFmt(h) {
    h = h.replace(/`([^`]+)`/g, '<code>$1</code>');
    h = h.replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
    h = h.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    h = h.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    return h;
  }
  function md(s) {
    var lines = esc(s == null ? '' : s).split('\n');
    var out = [];
    var inList = false;
    lines.forEach(function (line) {
      var m2 = line.match(/^##\s+(.*)/);
      var m1 = line.match(/^#\s+(.*)/);
      var ml = line.match(/^-\s+(.*)/);
      if (m2) {
        if (inList) { out.push('</ul>'); inList = false; }
        out.push('<h1 class="md-h2">' + inlineFmt(m2[1]) + '</h1>');
      } else if (m1) {
        if (inList) { out.push('</ul>'); inList = false; }
        out.push('<h1 class="md-h1">' + inlineFmt(m1[1]) + '</h1>');
      } else if (ml) {
        if (!inList) { out.push('<ul class="md-list">'); inList = true; }
        out.push('<li>' + inlineFmt(ml[1]) + '</li>');
      } else {
        if (inList) { out.push('</ul>'); inList = false; }
        if (/^\s*$/.test(line)) out.push('');
        else out.push(inlineFmt(line) + '<br>');
      }
    });
    if (inList) out.push('</ul>');
    var html = out.join('');
    return html.replace(/(<br>)+$/g, '').replace(/^(<br>)+/g, '') || '';
  }
  function favicon(url) {
    try { return 'https://www.google.com/s2/favicons?domain=' + new URL(url).hostname + '&sz=64'; }
    catch (e) { return ''; }
  }
  function host(url) { try { return new URL(url).hostname; } catch (e) { return url; } }

  // Column widths: localStorage only (no server prefs field). Returns {}.
  function getWidths() {
    try { return JSON.parse(localStorage.getItem(LS_WIDTHS) || '{}') || {}; }
    catch (e) { return {}; }
  }
  function setWidth(id, px) {
    try {
      var w = getWidths();
      w[id] = px;
      localStorage.setItem(LS_WIDTHS, JSON.stringify(w));
    } catch (e) {}
  }

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
    // Note blocks render even when empty (placeholder for brand-new cards);
    // link-only cards have no note block at all, so no placeholder appears.
    // No validation requires note content anywhere (client or server).
    if (b.type === 'note' || b.content) {
      var empty = !b.content;
      return '<div class="block note' + (empty ? ' empty' : '') + '" data-block="' + esc(b.id || '') + '"' +
        ' title="Click to edit">' + (empty ? 'Write something&hellip;' : md(b.content)) + '</div>';
    }
    return '';
  }

  function cardHTML(card) {
    // Link-only: blocks render as-is; only a card with zero blocks (or
    // blocks that render to '') falls back to the editable placeholder.
    var inner = card.blocks.map(blockHTML).join('') ||
      '<div class="block note empty" data-block="" title="Click to edit">Write something&hellip;</div>';
    // Compact ghost icon bar (no Edit button; editing is click-to-edit).
    return '<article class="card" draggable="true" data-card="' + card.id + '">' + inner +
      '<div class="cardbar"><button data-act="addlink" title="Add link">&#128279;</button>' +
      '<button data-act="addnote" title="Add note">&#9998;</button>' +
      '<button data-act="addimg" title="Upload image">&#128247;</button>' +
      '<button data-act="addimgurl" title="Add image URL">&#127760;</button>' +
      '<button data-act="del" title="Delete card">&times;</button></div></article>';
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
    // Board switcher with inline [+ board] chip after the last tab.
    boardsEl.innerHTML = state.boards.map(function (b) {
      return '<button data-board="' + b.id + '"' + (b.id === state.boardId ? ' class="active"' : '') + '>' + esc(b.title) + '</button>';
    }).join('') + '<button class="add-inline" data-newboard title="New board">+ </button>';
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
    var widths = getWidths();
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
    // Column strip with inline [+ add column] at the end (not at top).
    // New column titles auto-focus via openCreator('column').
    boardEl.innerHTML = head + '<div class="cols' + (focus ? ' focus' : '') + '">' + cols.map(function (col) {
      var cards = (tree.cards[col.id] || []).map(cardHTML).join('');
      var w = widths[col.id];
      var style = (w && !col.collapsed) ? ' style="width:' + w + 'px;flex-basis:' + w + 'px"' : '';
      return '<section class="column' + (col.collapsed ? ' collapsed' : '') + (focus && !col.collapsed ? ' reading' : '') + '" draggable="true" data-col="' + col.id + '" data-coldrag="' + col.id + '"' + style + '>' +
        '<h2><button class="fold" data-fold="' + col.id + '" title="' + (col.collapsed ? 'Expand' : 'Collapse') + '">' + (col.collapsed ? '▸' : '▾') + '</button>' +
        '<label class="cdot" style="background:' + esc(col.color || '#c9c4b6') + '" title="Column color">' +
        '<input type="color" data-colcolor="' + col.id + '" value="' + esc(col.color || '#4c8dff') + '" tabindex="-1"></label>' +
        '<span class="coltitle" data-coltitle="' + col.id + '" title="Click to rename">' + esc(col.title) + '</span>' +
        '<span class="colcount">' + (tree.cards[col.id] || []).length + '</span>' +
        '<span class="resize" data-resize="' + col.id + '" title="Resize column"></span></h2>' +
        '<div class="cards" data-cards="' + col.id + '">' + cards + '<button class="add-compact" data-add="' + col.id + '" title="Add card">+</button></div></section>';
    }).join('') + '<button class="add-col-inline" data-newcol title="Add column">+<span>add column</span></button></div>';
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
  function findCol(id) {
    var cols = (curTree() && curTree().columns) || [];
    for (var i = 0; i < cols.length; i++) if (cols[i].id === id) return cols[i];
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

  // If a card holds only a fresh empty note, a new link/image block
  // replaces it so link-only (or image-only) cards stay single-block.
  function pushOrReplace(card, blk) {
    if (card.blocks.length === 1 && card.blocks[0].type === 'note' && !card.blocks[0].content) {
      blk.id = card.blocks[0].id || blk.id;
      blk.position = 0;
      card.blocks = [blk];
    } else {
      blk.position = card.blocks.length;
      card.blocks.push(blk);
    }
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
  // New column titles auto-focus after creation.
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
          setTimeout(function () {
            var bt = document.getElementById('boardtitle');
            if (bt) startEdit(bt, true);
          }, 0);
        });
      } else if (state.boardId) {
        api('POST', '/api/boards/' + state.boardId + '/columns', { title: val, color: '#4c8dff' }).then(function (col) {
          var tree = curTree();
          if (tree) {
            tree.columns = (tree.columns || []).concat([col]);
            tree.cards[col.id] = [];
            saveCache(); render();
            // Auto-focus the new column title for immediate rename.
            setTimeout(function () {
              var el = boardEl.querySelector('[data-coltitle="' + col.id + '"]');
              if (el) startEdit(el, true);
            }, 0);
          }
          revalidate();
        });
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

  // ---- Deep-linkable boards: #/b/<id> (back/forward via hashchange) ----
  function boardFromURL() {
    var m = (location.hash || '').match(/^#\/b\/([A-Za-z0-9_-]+)/);
    if (m) return m[1];
    try {
      var q = new URLSearchParams(location.search).get('board');
      if (q) return q;
    } catch (e) {}
    return null;
  }
  function pushBoardURL(id) {
    var want = '#/b/' + id;
    if (location.hash !== want) {
      try { history.pushState(null, '', want); }
      catch (e) { location.hash = want; }
    }
  }
  window.addEventListener('hashchange', function () {
    var id = boardFromURL();
    if (id && id !== state.boardId && state.boards.some(function (b) { return b.id === id; })) selectBoard(id, true);
  });
  window.addEventListener('popstate', function () {
    var id = boardFromURL();
    if (id && id !== state.boardId && state.boards.some(function (b) { return b.id === id; })) selectBoard(id, true);
  });

  // ---- HTML5 drag-drop: cards within/across columns, columns reorder ----
  var dragCard = null, dragCol = null;
  document.addEventListener('dragstart', function (e) {
    var cardEl = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
    if (cardEl && !dragCol) {
      // Avoid starting a card drag from an active inline editor.
      if (e.target.isContentEditable) { e.preventDefault(); return; }
      dragCard = cardEl.dataset.card;
      try { e.dataTransfer.setData('text/plain', 'card:' + dragCard); e.dataTransfer.effectAllowed = 'move'; } catch (err) {}
      return;
    }
    var colEl = e.target && e.target.closest ? e.target.closest('[data-coldrag]') : null;
    if (colEl && (e.target === colEl || (e.target.closest && e.target.closest('h2')))) {
      if (e.target.isContentEditable || (e.target.closest && e.target.closest('button,input,label'))) return;
      dragCol = colEl.dataset.coldrag;
      try { e.dataTransfer.setData('text/plain', 'col:' + dragCol); e.dataTransfer.effectAllowed = 'move'; } catch (err) {}
    }
  });
  document.addEventListener('dragend', function () { dragCard = null; dragCol = null; });
  boardEl.addEventListener('dragover', function (e) {
    if (!dragCard && !dragCol) return;
    e.preventDefault();
    try { e.dataTransfer.dropEffect = 'move'; } catch (err) {}
  });
  boardEl.addEventListener('drop', function (e) {
    if (dragCol) {
      e.preventDefault();
      var over = e.target && e.target.closest ? e.target.closest('[data-coldrag]') : null;
      var tree = curTree();
      if (!tree || !over || over.dataset.coldrag === dragCol) { dragCol = null; return; }
      var ids = tree.columns.map(function (c) { return c.id; });
      var from = ids.indexOf(dragCol);
      var to = ids.indexOf(over.dataset.coldrag);
      if (from < 0 || to < 0) { dragCol = null; return; }
      var moved = tree.columns.splice(from, 1)[0];
      tree.columns.splice(to, 0, moved);
      tree.columns.forEach(function (c, i) { c.position = i; });
      saveCache(); render();
      tree.columns.forEach(function (c) { api('PATCH', '/api/columns/' + c.id, c).catch(function () {}); });
      dragCol = null;
      return;
    }
    if (dragCard) {
      e.preventDefault();
      var lane = e.target && e.target.closest ? e.target.closest('[data-cards]') : null;
      var onCard = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
      var toCol = lane ? lane.dataset.cards : (onCard ? (onCard.closest('[data-cards]') || {}).dataset.cards : null);
      // Fallback: dropping on a column section resolves its lane.
      if (!toCol) {
        var sec = e.target && e.target.closest ? e.target.closest('[data-col]') : null;
        if (sec) toCol = sec.dataset.col;
      }
      if (!toCol) { dragCard = null; return; }
      var tree2 = curTree();
      var cards = (tree2.cards[toCol] || []).filter(function (c) { return c.id !== dragCard; });
      var pos = cards.length;
      if (onCard && onCard.dataset.card !== dragCard) {
        for (var i = 0; i < cards.length; i++) if (cards[i].id === onCard.dataset.card) { pos = i; break; }
      }
      var moving = findCard(dragCard);
      var fromCol = moving ? moving.column_id : null;
      // Optimistic local move; server persists via existing move path.
      Object.keys(tree2.cards).forEach(function (k) {
        tree2.cards[k] = tree2.cards[k].filter(function (c) { return c.id !== dragCard; });
        tree2.cards[k].forEach(function (c, j) { c.position = j; });
      });
      if (moving) {
        moving.column_id = toCol;
        var dst = tree2.cards[toCol] || [];
        if (pos < 0 || pos > dst.length) pos = dst.length;
        dst.splice(pos, 0, moving);
        dst.forEach(function (c, j) { c.position = j; });
        tree2.cards[toCol] = dst;
      }
      saveCache(); render();
      api('POST', '/api/cards/' + dragCard + '/move', { column_id: toCol, position: pos })
        .then(revalidate).catch(function () {});
      // Keep collapse + per-board cache intact; revalidate merges server truth.
      void fromCol;
      dragCard = null;
    }
  });

  // ---- Resizable columns: drag handle, widths in localStorage ----
  boardEl.addEventListener('pointerdown', function (e) {
    var h = e.target && e.target.closest ? e.target.closest('[data-resize]') : null;
    if (!h) return;
    e.preventDefault();
    var id = h.dataset.resize;
    var sec = boardEl.querySelector('[data-col="' + id + '"]');
    if (!sec) return;
    var startX = e.clientX;
    var startW = sec.getBoundingClientRect().width;
    function mv(ev) {
      var w = Math.max(200, Math.min(900, Math.round(startW + (ev.clientX - startX))));
      sec.style.width = w + 'px';
      sec.style.flexBasis = w + 'px';
    }
    function up(ev) {
      document.removeEventListener('pointermove', mv);
      document.removeEventListener('pointerup', up);
      var w = Math.max(200, Math.min(900, Math.round(startW + (ev.clientX - startX))));
      setWidth(id, w);
      saveCache();
    }
    document.addEventListener('pointermove', mv);
    document.addEventListener('pointerup', up);
  });

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
    // Collapse all / expand all (persisted per column; never regressed).
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
    // Board title rename (inline; no Edit button).
    if (t.closest && t.closest('#boardtitle')) {
      var bt = document.getElementById('boardtitle');
      startEdit(bt, true);
      return;
    }
    // Column title rename (inline; no Edit button).
    var ct = t.closest && t.closest('[data-coltitle]');
    if (ct) { startEdit(ct, true); return; }
    if (t.dataset && t.dataset.fold) {
      // Collapse toggle: optimistic-local, synced behind.
      var colId = t.dataset.fold;
      var target = findCol(colId);
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
        pushOrReplace(card, { id: '', type: 'link', url: url, title: url, position: card.blocks.length });
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
          pushOrReplace(card, { id: '', type: 'image', image_url: up.url, thumb_url: up.thumb_url, position: card.blocks.length });
          syncCard(card); render();
        }).catch(function () { alert('Image upload failed (jpeg/png/gif, max 12MB).'); });
      };
      input.click();
    } else if (act === 'addimgurl') {
      askInCard(cardEl, 'Image URL', 'Paste image URL…').then(function (src) {
        if (!src) return;
        pushOrReplace(card, { id: '', type: 'image', image_url: src, position: card.blocks.length });
        syncCard(card); render();
      });
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
      var target = findCol(t.dataset.colcolor);
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
      var col = findCol(t.dataset.coltitle);
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
  // Enter commits an inline edit and exits edit mode; Ctrl/Cmd+Enter
  // inserts a newline. Applies to board/column titles and notes.
  // Escape cancels.
  function insertNewline(t) {
    try {
      if (document.execCommand && document.execCommand('insertText', false, '\n')) return;
    } catch (e) {}
    try {
      var sel = window.getSelection();
      if (!sel.rangeCount) return;
      var range = sel.getRangeAt(0);
      range.deleteContents();
      var br = document.createTextNode('\n');
      range.insertNode(br);
      range.setStartAfter(br);
      range.collapse(true);
      sel.removeAllRanges();
      sel.addRange(range);
    } catch (e) {}
  }
  boardEl.addEventListener('keydown', function (e) {
    var t = e.target;
    if (!t || !t.isContentEditable) return;
    if (e.key === 'Escape') { e.preventDefault(); t.blur(); render(); }
    else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      insertNewline(t);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      t.blur();
    }
  });

  boardsEl.addEventListener('click', function (e) {
    var nb = e.target.closest && e.target.closest('[data-newboard]');
    if (nb) { openCreator('board'); return; }
    var b = e.target.closest && e.target.closest('[data-board]');
    if (b) selectBoard(b.dataset.board);
  });

  function selectBoard(id, fromNav) {
    if (!id) return;
    state.boardId = id;
    saveCache();
    if (!fromNav) pushBoardURL(id);
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
    return api('GET', '/api/boards').then(function (boards) {
      state.boards = boards || [];
      var known = {};
      state.boards.forEach(function (b) { known[b.id] = true; });
      var deep = boardFromURL();
      if (deep && known[deep]) {
        state.boardId = deep;
      } else if (!state.boardId || !known[state.boardId]) {
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
  // Deep link wins over cached selection when it names a known board.
  // Boards list may still be loading; revalidate() re-applies it.
  var deep0 = boardFromURL();
  if (deep0) state.boardId = deep0;
  render();
  if (window.performance && performance.now) {
    window.__LOOM_FIRST_PAINT_MS = Math.round((performance.now() - t0) * 10) / 10;
  }
  // Background revalidation never blocks first paint.
  if (window.requestIdleCallback) requestIdleCallback(revalidate);
  else setTimeout(revalidate, 0);
})();
