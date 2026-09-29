'use strict';
/* Loom rebuild frontend (vanilla JS, no framework — initial bundle budget).
 * Instant-load: paint from localStorage cache synchronously, then
 * revalidate from network in the background. Server bootstrap
 * (window.__BOOTSTRAP_DATA__) seeds the cache on cold starts.
 * Writes are optimistic-local first, synced behind, server = truth.
 *
 * UX model (Columns.app feel): everything edits inline — double-click a
 * board tab, or click column titles or any note, to edit in place (no
 * Edit buttons).
 * Enter commits an edit; Shift/Ctrl/Cmd+Enter inserts a newline. New notes and
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
  // Round 4 item 6: board tab order override (array of board ids).
  // No server board-order path exists (PATCH /api/boards/{id} accepts
  // only {title,background}; ListBoards orders by position), so tab
  // order persists in localStorage only. Boards missing from the stored
  // array keep server order after the stored ones.
  var LS_BOARDORDER = 'loom.boardorder.v1';
  var boardEl = document.getElementById('board');
  var boardsEl = document.getElementById('boards');
  var creatorEl = document.getElementById('creator');
  var topbarCtl = document.getElementById('boardctl');
  var state = { boards: [], trees: {}, boardId: null };
  // Item 11 (white theme bug): Paper is an explicit 'paper' theme, never ''.
  // Root cause was SWATCHES id '' + CSS dark-mode guard keyed on "paper",
  // so selecting Paper left data-bg="" and the prefers-color-scheme:dark
  // rule kept applying. Empty/legacy values normalize to 'paper' on read.
  var BG_DEFAULT = 'paper';
  function normBg(bg) { return bg || 'paper'; }

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
  // Minimal markdown: "# H1", "## H2", "### H3", "**bold**", "*italic*", "`code`",
  // "[text](url)", "- list". No deps. Headings/lists are line-based;
  // inline marks apply within each line.
  function inlineFmt(h) {
    h = h.replace(/`([^`]+)`/g, '<code>$1</code>');
    h = h.replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g, '<a href="$2">$1</a>');
    h = h.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    h = h.replace(/(^|[\s(])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    return h;
  }
  function md(s) {
    var lines = esc(s == null ? '' : s).split('\n');
    var out = [];
    var inList = false;
    lines.forEach(function (line) {
      var m3 = line.match(/^###\s+(.*)/);
      var m2 = line.match(/^##\s+(.*)/);
      var m1 = line.match(/^#\s+(.*)/);
      var ml = line.match(/^-\s+(.*)/);
      if (m3) {
        if (inList) { out.push('</ul>'); inList = false; }
        out.push('<h3 class="md-h3">' + inlineFmt(m3[1]) + '</h3>');
      } else if (m2) {
        if (inList) { out.push('</ul>'); inList = false; }
        out.push('<h2 class="md-h2">' + inlineFmt(m2[1]) + '</h2>');
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
    try { return 'https://icons.duckduckgo.com/ip3/' + new URL(url).hostname + '.ico'; }
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

  function normalizeTree(tree) {    if (!tree) return null;
    if (!tree.columns) tree.columns = [];
    if (!tree.cards) tree.cards = {};
    tree.columns.forEach(function (c) {
      if (!tree.cards[c.id]) tree.cards[c.id] = [];
    });
    Object.keys(tree.cards).forEach(function (k) {
      (tree.cards[k] || []).forEach(normalizeCardTodos);
    });
    return tree;
  }

  // Single-block checklist model: one {type:'todo'} block holds the whole
  // list via its items array. Consecutive LEGACY single-row todo blocks
  // (no items) merge into one list here so cached/server data renders as
  // one <ul>; modern lists (with items) never merge, so adjacent lists
  // from + todo stay separate. The server applies the same rule on card
  // PATCH/read.
  function todoItemsOf(b) {
    if (b && Array.isArray(b.items) && b.items.length) return b.items;
    if (b && b.type === 'todo') {
      return [{ id: b.id || '', content: b.content || '', checked: !!b.checked }];
    }
    return [];
  }
  function normalizeCardTodos(card) {
    if (!card || !Array.isArray(card.blocks)) return card;
    var out = [], run = [];
    function flush() {
      if (!run.length) return;
      if (run.length === 1) {
        var b = run[0];
        b.items = todoItemsOf(b);
        b.content = '';
        b.checked = false;
        out.push(b);
      } else {
        var merged = { id: (run[0] && run[0].id) || '', type: 'todo', position: 0, items: [] };
        run.forEach(function (x) {
          todoItemsOf(x).forEach(function (it) { merged.items.push(it); });
        });
        out.push(merged);
      }
      run = [];
    }
    card.blocks.forEach(function (b) {
      if (b.type === 'todo' && !(b.items && b.items.length)) run.push(b);
      else { flush(); out.push(b); }
    });
    flush();
    out.forEach(function (b, i) { b.position = i; });
    card.blocks = out;
    return card;
  }
  function countTodoItems(card) {
    var n = 0;
    (card.blocks || []).forEach(function (b) {
      if (b.type === 'todo') n += todoItemsOf(b).length;
    });
    return n;
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
  // Client-minted temp ids for new todo blocks/items so Enter-chained
  // drafts match unambiguously before the server roundtrip assigns real
  // ids (the server keeps client ids; it only backfills empty ones).
  function tmpId() {
    return 'tmp-' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
  }
  // Round 4 item 6: ordered boards — server order (creation/position)
  // with the localStorage override applied. No server reorder path, so
  // this is the single source of tab order; see LS_BOARDORDER comment.
  function getBoardOrder() {
    try { var a = JSON.parse(localStorage.getItem(LS_BOARDORDER) || '[]'); return Array.isArray(a) ? a : []; }
    catch (e) { return []; }
  }
  function setBoardOrder(ids) {
    try { localStorage.setItem(LS_BOARDORDER, JSON.stringify(ids)); } catch (e) {}
  }
  function orderedBoards() {
    var order = getBoardOrder();
    var pos = {};
    order.forEach(function (id, i) { pos[id] = i; });
    return state.boards.slice().sort(function (a, b) {
      var pa = (a.id in pos) ? pos[a.id] : 1e9;
      var pb = (b.id in pos) ? pos[b.id] : 1e9;
      return pa - pb;
    });
  }
  function moveBoard(id, dir) {
    // Reorder within the full ordered list, then persist the full id
    // array so the override stays total (new boards append at the end).
    var ids = orderedBoards().map(function (b) { return b.id; });
    var i = ids.indexOf(id);
    var j = i + dir;
    if (i < 0 || j < 0 || j >= ids.length) return;
    ids.splice(i, 1);
    ids.splice(j, 0, id);
    setBoardOrder(ids);
    saveCache(); render();
  }
  // OSS-68: delete the current board behind an explicit confirm() —
  // a single click must never delete. Cancel changes nothing (no
  // network call). Confirm drops the board from client state without
  // a full reload and selects a remaining board (or empty state).
  function deleteBoard(id) {
    if (!id) return;
    var target = null;
    state.boards.forEach(function (b) { if (b.id === id) target = b; });
    var title = target ? target.title : id;
    if (!window.confirm('Delete board "' + title + '"? Its columns and cards go too. This cannot be undone.')) return;
    api('DELETE', '/api/boards/' + id).then(function () {
      state.boards = state.boards.filter(function (b) { return b.id !== id; });
      delete state.trees[id];
      try { setBoardOrder(getBoardOrder().filter(function (x) { return x !== id; })); } catch (e) {}
      if (state.boardId !== id) { saveCache(); render(); return; }
      var rest = orderedBoards();
      if (!rest.length) {
        state.boardId = null;
        try { history.pushState(null, '', location.pathname); } catch (e) {}
        saveCache(); render();
        return;
      }
      selectBoard(rest[0].id);
    }).catch(function () { alert('Delete failed — nothing was removed.'); });
  }
  var boardMenuOpen = false;

  // Inline Iconify-style icons (item 5): hand-picked 16px outline SVGs in
  // the spirit of the Iconify "link" sets (tabler/mdi lineage), inlined so
  // there is no new runtime dep and the gzip budget holds. The + affordance
  // for new card/board stays a text label (items 9/10) and is untouched.
  var ICONS = {
    link: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>',
    note: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>',
    img: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.09-3.09a2 2 0 0 0-2.82 0L6 21"/></svg>',
    globe: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M2 12h20"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>',
    del: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M18 6 6 18M6 6l12 12"/></svg>',
    todo: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="m9 12 2 2 4-4"/></svg>'
  };

  // Round 5 item 2: block drag handle — the ONLY block-drag zone
  // (card body drags the card, empty lane pans). Rendered only for
  // server-persisted blocks (non-empty id) so drag state always keys
  // on a real block id.
  function bhandleHTML(cardId, blockId) {
    if (!blockId) return '';
    return '<span class="bhandle" draggable="true" data-bcard="' + esc(cardId) + '" data-bid="' + esc(blockId) + '" title="Drag to move">⠿</span>';
  }

  function blockHTML(b, cardId) {
    // One todo block = one checklist <ul>; see todoListHTML below.
    if (b.type === 'todo') {
      return todoListHTML(b, cardId);
    }
    if (b.type === 'link' && b.url) {
      var fav = favicon(b.url);
      var h = host(b.url);
      var letter = esc((h.charAt(0) || '?').toUpperCase());
      // Item 7: small remove affordance on link blocks, persisted via card PATCH.
      return '<div class="block block-link">' + bhandleHTML(cardId, b.id) + '<a class="link" href="' + esc(b.url) + '">' +
        (fav ? '<img src="' + fav + '" alt="" loading="lazy" width="22" height="22" data-letter="' + letter + '">' : '') +
        '<span><span class="t">' + esc(b.title || b.url) + '</span><br><span class="u">' + esc(host(b.url)) + '</span></span></a>' +
        '<button class="rmblock" data-rmblock="' + esc(b.id || '') + '" title="Remove link" aria-label="Remove link">&times;</button></div>';
    }
    if (b.type === 'image' && (b.thumb_url || b.image_url)) {
      var src = b.thumb_url || b.image_url;
      var full = b.image_url || b.thumb_url;
      // Item 7: small remove affordance on image blocks, persisted via card PATCH.
      return '<div class="block block-img">' + bhandleHTML(cardId, b.id) + '<a href="' + esc(full) + '">' +
        '<img class="photo" src="' + esc(src) + '" alt="' + esc(b.alt || '') + '" loading="lazy" decoding="async"></a>' +
        '<button class="rmblock" data-rmblock="' + esc(b.id || '') + '" title="Remove image" aria-label="Remove image">&times;</button></div>';
    }
    // Item 1 (empty-note whitespace): empty notes render as a zero-layout
    // inline affordance, not an empty block. Link-only cards carry no note
    // block at all, so nothing renders until click-to-add-text.
    // No validation requires note content anywhere (client or server).
    if (b.type === 'note' || b.content) {
      if (!b.content) {
        return '<button class="note-add" data-block="' + esc(b.id || '') + '" title="Add text">+ add text</button>';
      }
      return '<div class="block note" data-block="' + esc(b.id || '') + '"' +
        ' title="Click to edit">' + bhandleHTML(cardId, b.id) + md(b.content) + '</div>';
    }
    return '';
  }

  // Todo-list blocks: one block holds the whole checklist via its items
  // array and renders as one <ul>. The checkbox toggles checked via card
  // PATCH without entering edit mode; the text span edits inline. The <ul>
  // itself carries .block + data-block so block drag-drop targets the list.
  function todoItemHTML(block, it, cardId) {
    void cardId;
    var label = it.content ? esc(it.content) : '<span class="todo-empty">Todo…</span>';
    var bid = esc(block.id || ''), iid = esc(it.id || '');
    return '<li class="todo' + (it.checked ? ' done' : '') + '" data-todolist="' + bid + '" data-todo="' + iid + '">' +
      '<input type="checkbox" data-todolist="' + bid + '" data-todotoggle="' + iid + '"' + (it.checked ? ' checked' : '') + ' aria-label="Toggle todo">' +
      '<span class="todo-text" data-todolist="' + bid + '" data-todo="' + iid + '" title="Click to edit">' + label + '</span>' +
      '<button class="rmblock" data-todolist="' + bid + '" data-rmtodo="' + iid + '" title="Remove todo" aria-label="Remove todo">&times;</button></li>';
  }

  function todoListHTML(b, cardId) {
    return '<ul class="todo-list block" data-block="' + esc(b.id || '') + '">' + bhandleHTML(cardId, b.id) +
      todoItemsOf(b).map(function (it) { return todoItemHTML(b, it, cardId); }).join('') + '</ul>';
  }

  // Each todo block renders its own <ul> checklist; no cross-block
  // grouping (one block = one list; a second list needs + todo).
  function blocksHTML(card) {
    return card.blocks.map(function (b) { return blockHTML(b, card.id); }).join('');
  }

  function cardHTML(card) {
    // Item 1: a card with zero blocks (or blocks rendering to '') gets a
    // zero-layout add-text affordance, never an empty note div. Tabindex
    // makes first-tap reveal the hover toolbar on touch (item 2).
    var inner = blocksHTML(card) ||
      '<button class="note-add" data-block="" title="Add text">+ add text</button>';
    // Item 2: floating overlay toolbar (CSS absolute, no layout shift),
    // revealed on hover / focus-within / first tap (.showbar). Item 5:
    // Iconify-style inline SVG icons. Item 6: delete X far right with
    // red hover + title/aria-label. Item 10 handled in column lane below.
    return '<article class="card" draggable="true" tabindex="0" data-card="' + card.id + '">' + inner +
      '<div class="cardbar" role="toolbar" aria-label="Card actions">' +
      '<button data-act="addlink" title="Add link" aria-label="Add link">' + ICONS.link + '</button>' +
      '<button data-act="addnote" title="Add note" aria-label="Add note">' + ICONS.note + '</button>' +
      '<button data-act="addtodo" title="Add todo" aria-label="Add todo">' + ICONS.todo + '</button>' +
      '<button data-act="addimg" title="Upload image" aria-label="Upload image">' + ICONS.img + '</button>' +
      '<button data-act="addimgurl" title="Add image URL" aria-label="Add image URL">' + ICONS.globe + '</button>' +
      '<button data-act="del" class="del" title="Delete card" aria-label="Delete card">' + ICONS.del + '</button></div></article>';
  }

  var SWATCHES = [
    { id: 'paper', name: 'Paper' },
    { id: 'honey', name: 'Honey' },
    { id: 'sage', name: 'Sage' },
    { id: 'sky', name: 'Sky' },
    { id: 'rose', name: 'Rose' },
    { id: 'slate', name: 'Ink' },
  ];

  function render() {
    // OSS-43: mutating column buttons all end in render(), which rebuilds
    // boardEl.innerHTML including the .cols strip — destroying the scroller
    // resets scrollLeft to 0. Capture/restore here so every caller is covered.
    var prevStrip = boardEl.querySelector('.cols');
    var savedScroll = prevStrip ? prevStrip.scrollLeft : 0;
    // (static HTML holds the wordmark; tabs render here). Right zone =
    // #boardctl (expand/collapse + theme) followed by .actions (auth +
    // Settings) clustered far-right via a single margin-left:auto — no
    // board-name repeat, no centered cluster.
    // Round 4 item 6: with >3 boards show the first three in creation
    // order plus a dropdown next to the third listing ALL boards with
    // reorder controls (localStorage order; see LS_BOARDORDER).
    var ordered = orderedBoards();
    var visible = ordered.length > 3 ? ordered.slice(0, 3) : ordered;
    // Round 6 item 3: the tabs scroll inside an inner .boardtabs element
    // while the overflow menu (.boardmenu-wrap) sits OUTSIDE it as a
    // direct child of #boards — navbar scrolling can never clip the menu.
    boardsEl.innerHTML = '<span class="boardtabs">' + visible.map(function (b) {
      return '<button data-board="' + b.id + '"' + (b.id === state.boardId ? ' class="active"' : '') +
        ' title="Open board (double-click to rename)">' + esc(b.title) + '</button>';
    }).join('') + '</span>' +
      (ordered.length > 3
        ? '<span class="boardmenu-wrap"><button class="boardmenu-btn" data-boardmenu aria-haspopup="true" title="All boards">▾</button>' +
          (boardMenuOpen ? '<div class="boardmenu" role="menu">' + ordered.map(function (b) {
            return '<div class="boardmenu-row' + (b.id === state.boardId ? ' current' : '') + '">' +
              '<button class="boardmenu-go" data-board="' + b.id + '" role="menuitem">' + esc(b.title) + '</button>' +
              '<button class="mv" data-boardup="' + b.id + '" title="Move up" aria-label="Move ' + esc(b.title) + ' up">↑</button>' +
              '<button class="mv" data-boarddown="' + b.id + '" title="Move down" aria-label="Move ' + esc(b.title) + ' down">↓</button></div>';
          }).join('') + '</div>' : '') + '</span>'
        : '') +
      '<button class="add-inline" data-newboard title="New board">+ </button>' +
      // OSS-68: delete affordance for the current board, next to +.
      // Confirm-gated in deleteBoard(); hidden with no selection.
      (state.boardId
        ? '<button class="del-board" data-delboard="' + state.boardId + '" title="Delete this board" aria-label="Delete this board">×</button>'
        : '');
    var tree = curTree();
    if (!tree) {
      document.body.dataset.bg = BG_DEFAULT;
      topbarCtl.innerHTML = '';
      boardEl.innerHTML = state.boards.length
        ? '<div class="emptyboard"><p>Select a board above, or create one to start writing.</p></div>'
        : '<div class="emptyboard"><p>Welcome to Loom — create your first board to start writing.</p><button class="primary" data-newboard>＋ New board</button></div>';
      return;
    }
    var bg = normBg(tree.board.background);
    document.body.dataset.bg = bg;
    var widths = getWidths();
    var cols = tree.columns || [];
    var open = cols.filter(function (c) { return !c.collapsed; });
    var focus = open.length === 1 && cols.length > 1;
    var allFolded = cols.length > 0 && open.length === 0;
    // Round 4 item 2: right zone order = expand-all/collapse-all, then
    // theme switcher (background swatches). Language (EN slot) and the v1
    // importer live in Settings (OSS-57). The board title is NOT repeated
    // here (rename via double-click on the active board tab).
    topbarCtl.innerHTML =
      (cols.length ? '<button class="ghost compact" data-foldall>' + (allFolded ? 'Expand all' : 'Collapse all') + '</button>' : '') +
      (state.auth.enabled ? '<button class="ghost compact" data-share title="Visibility + members">Share</button>' : '') +
      '<span class="swatches" role="group" aria-label="Board background">' +
      SWATCHES.map(function (s) {
        return '<button data-bg="' + s.id + '"' + (bg === s.id ? ' class="on"' : '') +
          ' title="' + s.name + '" aria-label="' + s.name + ' background"><i class="sw sw-' + s.id + '"></i></button>';
      }).join('') + '</span>';
    if (!cols.length) {
      boardEl.innerHTML = '<div class="emptyboard"><p>This board has no columns yet.</p><button class="primary" data-newcol>Add your first column</button></div>';
      return;
    }
    // Column strip with inline [+ add column] at the end (not at top).
    // New column titles auto-focus via openCreator('column').
    boardEl.innerHTML = '<div class="cols' + (focus ? ' focus' : '') + '">' + cols.map(function (col) {
      var cards = (tree.cards[col.id] || []).map(cardHTML).join('');
      var w = widths[col.id];
      // Item 8: column color themes the column via --colc (header wash +
      // accent border in CSS); keeps working in light/dark via color-mix.
      var style = '--colc:' + esc(col.color || '#c9c4b6') + ';' +
        ((w && !col.collapsed) ? 'width:' + w + 'px;flex-basis:' + w + 'px;' : '');
      // OSS-40: only the header is draggable — the section keeps
      // data-coldrag as a drop target, but native drag never starts from
      // lane background / section chrome, so pointer pan survives.
      return '<section class="column' + (col.collapsed ? ' collapsed' : '') + (focus && !col.collapsed ? ' reading' : '') + '" data-col="' + col.id + '" data-coldrag="' + col.id + '"' + ' style="' + style + '">' +
        '<h2 draggable="true"><button class="fold" data-fold="' + col.id + '" title="' + (col.collapsed ? 'Expand' : 'Collapse') + '">' + (col.collapsed ? '▸' : '▾') + '</button>' +
        '<button class="cdot" data-coldot="' + col.id + '" style="background:' + esc(col.color || '#c9c4b6') + '" title="Column color" aria-label="Column color"></button>' +
        '<span class="coltitle" data-coltitle="' + col.id + '" title="Click to rename">' + esc(col.title) + '</span>' +
        '<span class="colcount">' + (tree.cards[col.id] || []).length + '</span>' +
        '<span class="resize" data-resize="' + col.id + '" title="Resize column"></span></h2>' +
        // Item 10: explicit "+ add card" text label, not a bare +.
        '<div class="cards" data-cards="' + col.id + '">' + cards + '<button class="add-compact" data-add="' + col.id + '" title="Add card">+ add card</button></div></section>';
    }).join('') + '<button class="add-col-rail" data-newcol title="Add column"><span aria-hidden="true">+</span><span class="rail-label">add column</span></button></div>';
    // OSS-43: restore synchronously in the same task — no flicker, and this
    // runs outside any pointer-pan gesture so it never fights panState.strip.
    if (savedScroll) {
      var strip = boardEl.querySelector('.cols');
      if (strip) strip.scrollLeft = savedScroll;
    }
    wireFavFallbacks();
  }

  // Favicon fallback: a failed link-icon <img> swaps itself for a letter
  // tile, so a missing icon never shows a broken-image glyph. Wired per
  // element in render() plus a document capture listener as backup.
  // data-letter is only set on link favicon imgs, so photo blocks are
  // unaffected. Re-wired on every render since innerHTML replaces nodes.
  function favFallback(img) {
    if (!img || img.tagName !== 'IMG' || !img.dataset || !img.dataset.letter || !img.isConnected) return;
    var s = document.createElement('span');
    s.className = 'fav-fallback';
    s.textContent = img.dataset.letter;
    img.replaceWith(s);
  }
  document.addEventListener('error', function (e) { favFallback(e.target); }, true);
  function wireFavFallbacks() {
    boardEl.querySelectorAll('img[data-letter]').forEach(function (img) {
      img.addEventListener('error', function () { favFallback(img); });
    });
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
  // Returns the sync promise so dependent writes (block cross-card
  // moves) can chain in server-safe order.
  function syncCard(card) {
    saveCache();
    return api('PATCH', '/api/cards/' + card.id, card).then(function (fresh) {
      Object.assign(card, fresh);
      saveCache(); renderPreservingTodoFocus();
    }).catch(function () { /* stays local; revalidates next load */ });
  }

  // Re-render without dropping an in-progress todo edit: a PATCH
  // resolving after Enter-chained focus would otherwise rebuild the DOM
  // and leave the new row unfocused. Captures the focused row by id and
  // restores the caret to the end of the same row after render.
  function renderPreservingTodoFocus() {
    var a = document.activeElement;
    var memo = null;
    if (a && a.isContentEditable && a.isConnected && a.classList &&
        a.classList.contains('todo-text') && a.dataset && a.dataset.todo) {
      var ce = a.closest && a.closest('[data-card]');
      if (ce && ce.dataset.card) {
        memo = { cardId: ce.dataset.card, list: a.dataset.todolist || '', item: a.dataset.todo };
      }
    }
    render();
    if (!memo) return;
    var cardEl = boardEl.querySelector('[data-card="' + memo.cardId + '"]');
    if (!cardEl) return;
    var spans = cardEl.querySelectorAll('.todo-text');
    for (var i = 0; i < spans.length; i++) {
      if ((spans[i].dataset.todo || '') === memo.item &&
          (spans[i].dataset.todolist || '') === memo.list) {
        var c = findCard(memo.cardId);
        var loc = c ? todoLoc(c, memo.list, memo.item) : null;
        if (loc && loc.item) editTodoRaw(spans[i], loc.item, false);
        return;
      }
    }
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

  // Item 3 (editable markdown source): re-editing swaps the rendered HTML
  // for the RAW markdown source first, so render applies only on commit and
  // nothing is lost round-tripping through md().
  function editNoteRaw(noteEl, blk, selectAll) {
    if (noteEl && blk && typeof blk.content === 'string') {
      noteEl.textContent = blk.content;
    }
    startEdit(noteEl, selectAll);
  }

  function blockById(card, id) {
    if (!card) return null;
    for (var i = 0; i < card.blocks.length; i++) {
      if ((card.blocks[i].id || '') === (id || '')) return card.blocks[i];
    }
    return null;
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
      var notes = cardEl.querySelectorAll('.note,.note-add');
      target = notes.length ? notes[notes.length - 1] : null;
    }
    if (!target) { cardEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' }); return; }
    // Item 1: the zero-layout affordance is a <button>; convert to an
    // editable note div in place, then edit raw source (item 3).
    if (target.classList.contains('note-add')) {
      var card = findCard(cardId);
      var blk = card && blockById(card, target.dataset.block);
      if (!blk && card) {
        blk = { id: target.dataset.block || '', type: 'note', content: '', position: card.blocks.length };
        card.blocks.push(blk);
        saveCache(); render();
        focusNote(cardId, blk.id, selectAll);
        return;
      }
      var div = document.createElement('div');
      div.className = 'block note editing';
      div.dataset.block = target.dataset.block || '';
      div.title = 'Click to edit';
      div.textContent = (blk && blk.content) || '';
      target.replaceWith(div);
      startEdit(div, selectAll !== false);
      return;
    }
    var c = findCard(cardId);
    editNoteRaw(target, c && blockById(c, target.dataset.block), selectAll !== false);
  }

  // Todo inline edit: the text span edits raw content like notes. Locate
  // an item by (list block id, item id); ensures the block carries a real
  // items array so edits always land in the same block. Blur commits;
  // committing empty text deletes that item only, keeping the rest.
  function todoLoc(card, blockId, itemId) {
    if (!card) return null;
    for (var i = 0; i < card.blocks.length; i++) {
      var b = card.blocks[i];
      if (b.type !== 'todo') continue;
      if ((blockId || '') !== '' && (b.id || '') !== (blockId || '')) continue;
      if (!Array.isArray(b.items) || !b.items.length) b.items = todoItemsOf(b);
      for (var j = 0; j < b.items.length; j++) {
        if ((b.items[j].id || '') === (itemId || '')) {
          return { block: b, index: i, item: b.items[j], itemIndex: j };
        }
      }
      if ((itemId || '') === '') {
        for (var k = b.items.length - 1; k >= 0; k--) {
          if (!b.items[k].content) return { block: b, index: i, item: b.items[k], itemIndex: k };
        }
      }
      if ((blockId || '') !== '' && (b.id || '') === (blockId || '')) {
        return { block: b, index: i, item: null, itemIndex: -1 };
      }
    }
    return null;
  }

  // Global item ordinal across all todo lists in a card (for focus).
  function todoOrdinal(card, blockId, itemId) {
    var n = 0;
    for (var i = 0; i < card.blocks.length; i++) {
      if (card.blocks[i].type !== 'todo') continue;
      var items = todoItemsOf(card.blocks[i]);
      for (var j = 0; j < items.length; j++) {
        if ((card.blocks[i].id || '') === (blockId || '') && (items[j].id || '') === (itemId || '')) return n;
        n++;
      }
    }
    return n;
  }

  // Drop one item; when its list empties, drop the list block too.
  function removeTodoItem(card, loc) {
    loc.block.items.splice(loc.itemIndex, 1);
    if (!loc.block.items.length) {
      card.blocks.splice(loc.index, 1);
    }
    card.blocks.forEach(function (b, i) { b.position = i; });
  }

  // The Todo… placeholder is render-only markup, never model content:
  // always reset the span to the raw item text (empty string for a new
  // row) before editing, so the placeholder can never become typed text.
  function editTodoRaw(spanEl, item, selectAll) {
    if (spanEl) {
      spanEl.textContent = (item && typeof item.content === 'string') ? item.content : '';
    }
    startEdit(spanEl, selectAll);
  }

  function commitTodo(el) {
    var cardEl = el.closest && el.closest('[data-card]');
    if (!cardEl) return;
    var card = findCard(cardEl.dataset.card);
    if (!card) return;
    var loc = todoLoc(card, el.dataset.todolist, el.dataset.todo);
    if (!loc || !loc.item) { render(); return; }
    var text = el.innerText.replace(/\n+$/, '');
    if (!text) {
      // Empty todo commits as a delete of that item only.
      removeTodoItem(card, loc);
      syncCard(card); render();
      return;
    }
    if ((loc.item.content || '') !== text) {
      loc.item.content = text;
      syncCard(card);
    }
    render();
  }

  // Focus the todo text at global item ordinal right after a render
  // commit. Ordinal-based (not id-based) so Enter-chained drafts focus
  // exactly even before the server assigns ids.
  function focusTodoAt(cardId, ordinal, selectAll) {
    var cardEl = boardEl.querySelector('[data-card="' + cardId + '"]');
    if (!cardEl) return;
    var spans = cardEl.querySelectorAll('.todo-text');
    var target = spans.length ? spans[Math.max(0, Math.min(ordinal, spans.length - 1))] : null;
    if (!target) { cardEl.scrollIntoView({ block: 'nearest', behavior: 'smooth' }); return; }
    var c = findCard(cardId);
    var loc = c ? todoLoc(c, target.dataset.todolist, target.dataset.todo) : null;
    editTodoRaw(target, loc && loc.item, selectAll !== false);
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
        // New boards inherit the current board's background (OSS-58).
        var tree0 = curTree();
        var bg0 = (tree0 && tree0.board && tree0.board.background) || undefined;
        api('POST', '/api/boards', bg0 ? { title: val, background: bg0 } : { title: val }).then(function (nb) {
          state.boards.push(nb);
          state.trees[nb.id] = normalizeTree({ board: nb, columns: [], cards: {} });
          selectBoard(nb.id);
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
  // Item 12: add-column form renders INLINE at the strip end where the
  // column will appear (not a top-of-window field). Commit creates the
  // column in place; Escape cancels.
  function openInlineColForm() {
    closeCreator();
    closeInlineColForm();
    var strip = boardEl.querySelector('.cols');
    var rail = boardEl.querySelector('.add-col-rail');
    var form = document.createElement('div');
    form.className = 'inline-col-form';
    form.innerHTML = '<input type="text" placeholder="Column name…" aria-label="New column name">' +
      '<button data-ok>Create</button><button data-cancel aria-label="Cancel">✕</button>';
    if (strip) strip.insertBefore(form, rail || null);
    else { creatorEl.appendChild(form); }
    var input = form.querySelector('input');
    input.focus();
    function done(commit) {
      var val = input.value.trim();
      var go = commit && val && state.boardId;
      form.remove();
      if (!go) return;
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
    form.querySelector('[data-ok]').addEventListener('click', function () { done(true); });
    form.querySelector('[data-cancel]').addEventListener('click', function () { done(false); });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') done(true);
      else if (e.key === 'Escape') done(false);
    });
    input.addEventListener('blur', function () {
      // Commit on blur only if a name was typed; otherwise just close.
      setTimeout(function () { if (form.isConnected) done(!!input.value.trim()); }, 0);
    });
  }
  function closeInlineColForm() {
    var f = boardEl.querySelector && boardEl.querySelector('.inline-col-form');
    if (f) f.remove();
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

  // ---- HTML5 drag-drop: cards within/across columns, columns reorder,
  // ---- blocks within/across cards (Round 5 item 2) ----
  var dragCard = null, dragCol = null, dragBlock = null;
  document.addEventListener('dragstart', function (e) {
    // Round 5 item 2: the block handle is checked FIRST so a block drag
    // never starts a card drag (distinct handle zone, no gesture conflict).
    var bh = e.target && e.target.closest ? e.target.closest('.bhandle') : null;
    if (bh && bh.dataset.bid) {
      dragBlock = { card: bh.dataset.bcard, block: bh.dataset.bid };
      try { e.dataTransfer.setData('text/plain', 'block:' + dragBlock.card + ':' + dragBlock.block); e.dataTransfer.effectAllowed = 'move'; } catch (err) {}
      return;
    }
    var cardEl = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
    if (cardEl && !dragCol) {
      // Avoid starting a card drag from an active inline editor.
      if (e.target.isContentEditable) { e.preventDefault(); return; }
      dragCard = cardEl.dataset.card;
      try { e.dataTransfer.setData('text/plain', 'card:' + dragCard); e.dataTransfer.effectAllowed = 'move'; } catch (err) {}
      return;
    }
    // OSS-40: column reorder starts ONLY from the header (h2[draggable]).
    // Any stray section-level dragstart (lane background / section chrome)
    // is cancelled so the native drag lifecycle never fires pointercancel
    // and kill the pointer pan. The section keeps data-coldrag as the sole
    // drop target so dragover/drop hints still land on the full column.
    var colHead = e.target && e.target.closest ? e.target.closest('h2') : null;
    var colSec = e.target && e.target.closest ? e.target.closest('[data-coldrag]') : null;
    if (colHead && colSec && colHead.closest('[data-coldrag]') === colSec) {
      if (e.target.isContentEditable || (e.target.closest && e.target.closest('button,input,label'))) return;
      dragCol = colSec.dataset.coldrag;
      try { e.dataTransfer.setData('text/plain', 'col:' + dragCol); e.dataTransfer.effectAllowed = 'move'; } catch (err) {}
      return;
    }
    if (colSec && !cardEl && !bh) { try { e.preventDefault(); } catch (err) {} }
  });
  document.addEventListener('dragend', function () { dragCard = null; dragCol = null; dragBlock = null; clearDropHints(); });
  // Item 13: visible drop zones — insertion line/highlight in lanes and
  // between columns during the drag, via native drag events. Hints clear
  // on every dragover reset, drop, and dragend; no silent reordering.
  // Round 5 item 2 adds block insertion lines (.drop-bbefore/.drop-bafter)
  // plus a dashed outline on the hovered card (.drop-bhint).
  function clearDropHints() {
    boardEl.querySelectorAll('.drop-hint,.drop-before,.drop-col-before,.drop-col-after,.drop-bbefore,.drop-bafter,.drop-bhint').forEach(function (el) {
      el.classList.remove('drop-hint', 'drop-before', 'drop-col-before', 'drop-col-after', 'drop-bbefore', 'drop-bafter', 'drop-bhint');
    });
  }
  boardEl.addEventListener('dragover', function (e) {
    if (!dragCard && !dragCol && !dragBlock) return;
    e.preventDefault();
    try { e.dataTransfer.dropEffect = 'move'; } catch (err) {}
    clearDropHints();
    if (dragBlock) {
      var cEl = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
      // Round 6 item 1: uniform drop zones — ANY part of a .block (body,
      // link anchor, image, note text), not just the ⠿ handle, shows the
      // before/after insertion line by pointer half, for all block types.
      var blkEl = e.target && e.target.closest ? e.target.closest('.block') : null;
      var tBid = null;
      if (blkEl) {
        var hh = blkEl.querySelector('.bhandle');
        tBid = (hh && hh.dataset.bid) || (blkEl.dataset && blkEl.dataset.block) || null;
      }
      if (blkEl && tBid && !(cEl && cEl.dataset.card === dragBlock.card && tBid === dragBlock.block)) {
        var rr = blkEl.getBoundingClientRect();
        blkEl.classList.add((e.clientY - rr.top) < rr.height / 2 ? 'drop-bbefore' : 'drop-bafter');
      }
      if (cEl) cEl.classList.add('drop-bhint');
      return;
    }
    if (dragCard) {
      var onCard = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
      var lane = e.target && e.target.closest ? e.target.closest('[data-cards]') : null;
      if (onCard && onCard.dataset.card !== dragCard) onCard.classList.add('drop-before');
      if (lane) lane.classList.add('drop-hint');
      else {
        var sec = e.target && e.target.closest ? e.target.closest('[data-col]') : null;
        if (sec) {
          var l = sec.querySelector('[data-cards]');
          if (l) l.classList.add('drop-hint');
        }
      }
    } else if (dragCol) {
      var over = e.target && e.target.closest ? e.target.closest('[data-coldrag]') : null;
      if (over && over.dataset.coldrag !== dragCol) {
        var r = over.getBoundingClientRect();
        var after = (e.clientX - r.left) > r.width / 2;
        over.classList.add(after ? 'drop-col-after' : 'drop-col-before');
      }
    }
  });
  boardEl.addEventListener('dragleave', function (e) {
    // Only clear when truly leaving the board surface, not moving between
    // children (relatedTarget still inside boardEl).
    if (!e.relatedTarget || !(e.relatedTarget instanceof Node) || !boardEl.contains(e.relatedTarget)) clearDropHints();
  });
  boardEl.addEventListener('drop', function (e) {
    clearDropHints();
    // Round 5 item 2: block move within/across cards, persisted via the
    // existing card PATCH path (syncCard). Round 6 item 1: dropping on
    // ANY part of a block inserts before/after by pointer half, uniformly
    // for link/note/image; dropping on card chrome appends.
    if (dragBlock) {
      e.preventDefault();
      var src = findCard(dragBlock.card);
      var dstEl = e.target && e.target.closest ? e.target.closest('[data-card]') : null;
      var dst = dstEl ? findCard(dstEl.dataset.card) : null;
      if (!src || !dst) { dragBlock = null; return; }
      var from = -1, bi;
      for (bi = 0; bi < src.blocks.length; bi++) {
        if ((src.blocks[bi].id || '') === dragBlock.block) { from = bi; break; }
      }
      if (from < 0) { dragBlock = null; return; }
      var bT = e.target && e.target.closest ? e.target.closest('.block') : null;
      var hIn = bT ? bT.querySelector('.bhandle') : null;
      var hBid = (hIn && hIn.dataset.bid) || (bT && bT.dataset && bT.dataset.block) || null;
      if (bT && hBid === dragBlock.block) { dragBlock = null; return; }
      var to = dst.blocks.length, bj;
      if (bT && hBid) {
        var tr = bT.getBoundingClientRect();
        var beforeT = (e.clientY - tr.top) < tr.height / 2;
        for (bj = 0; bj < dst.blocks.length; bj++) {
          if ((dst.blocks[bj].id || '') === hBid) { to = bj + (beforeT ? 0 : 1); break; }
        }
      }
      var mvb = src.blocks.splice(from, 1)[0];
      if (src === dst) {
        if (to === from || to === from + 1) { src.blocks.splice(from, 0, mvb); dragBlock = null; return; }
        if (to > from) to--;
      }
      if (to < 0 || to > dst.blocks.length) to = dst.blocks.length;
      dst.blocks.splice(to, 0, mvb);
      src.blocks.forEach(function (b, i) { b.position = i; });
      if (src !== dst) dst.blocks.forEach(function (b, i) { b.position = i; });
      // Cross-card moves chain src-then-dst: the block row must leave
      // the source before joining the destination (UNIQUE blocks.id).
      if (src !== dst) {
        syncCard(src).then(function () { syncCard(dst); });
      } else {
        syncCard(dst);
      }
      render();
      dragBlock = null;
      return;
    }
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

  // ---- Drag-to-scroll: pointer-drag on empty canvas pans horizontally ----
  // (round 3 item 6; mouse + touch). Starts ONLY on empty canvas area —
  // never on columns, cards, or any control — so it coexists with
  // card/column HTML5 drag and the resize handle. A click landing right
  // after a pan is swallowed so it never triggers a board action.
  // Round 4 item 1: the column-drag handle zone ends at the "+ add card"
  // row — empty lane area BELOW that row pans the strip horizontally
  // (existing pan behavior), never drags the column. Column HTML5 drag
  // itself starts only from the header (h2), so this is purely a pan
  // targeting rule: pointer events below the add-button's bottom edge
  // inside an open lane count as empty canvas.
  var panState = null, panMovedAt = 0;
  function panEmptyTarget(t, clientY) {
    if (!t || !t.closest) return false;
    if (!t.closest('.cols')) return t === boardEl;
    if (!t.closest('.column,.card,button,input,a,select,textarea,.inline-col-form,.composer')) return true;
    // Round 4 item 1: inside a lane but below the "+ add card" row, on
    // the lane background itself (not on a card/control), pans.
    var lane = t.closest && t.closest('[data-cards]');
    if (lane && t === lane) {
      var add = lane.querySelector('[data-add]');
      if (add && clientY !== undefined) {
        try {
          if (clientY > add.getBoundingClientRect().bottom) return true;
        } catch (e) {}
      }
    }
    // Round 6 item 2: open columns hug content, so the strip background
    // below a short column pans via the empty-canvas rule above — but a
    // pointer landing on the column section itself (chrome around the
    // lane, below the add-card row) pans too, never drags the column.
    var sec = t.closest && t.closest('[data-col]');
    if (sec && t === sec) return true;
    return false;
  }
  boardEl.addEventListener('pointerdown', function (e) {
    if (e.isPrimary === false) return;
    if (e.button !== undefined && e.button !== 0) return;
    if (!panEmptyTarget(e.target, e.clientY)) return;
    var strip = (e.target.closest && e.target.closest('.cols')) || boardEl.querySelector('.cols');
    panState = { strip: strip, x: e.clientX, active: false };
  });
  document.addEventListener('pointermove', function (e) {
    if (!panState || e.isPrimary === false) return;
    var dx = e.clientX - panState.x;
    if (!panState.active) {
      if (Math.abs(dx) < 5) return;
      panState.active = true;
      if (panState.strip) panState.strip.classList.add('panning');
    }
    if (panState.strip) panState.strip.scrollLeft -= dx;
    panState.x = e.clientX;
    panMovedAt = Date.now();
  });
  function endPan() {
    if (panState && panState.strip) panState.strip.classList.remove('panning');
    panState = null;
  }
  document.addEventListener('pointerup', endPan);
  document.addEventListener('pointercancel', endPan);

  boardEl.addEventListener('click', function (e) {
    var t = e.target;
    // Swallow the click that lands right after a canvas pan (item 6).
    if (panMovedAt && Date.now() - panMovedAt < 350) return;
    // Round 5 item 2: the block drag handle is a drag-only zone — its
    // click (no movement) must not start note editing or any action.
    if (t.closest && t.closest('.bhandle')) return;
    if (t.closest && t.closest('[data-newboard]')) { openCreator('board'); return; }
    if (t.closest && t.closest('[data-newcol]')) { openInlineColForm(); return; }
    // Column color dot opens the presets + custom popover (item 4).
    var dot = t.closest && t.closest('[data-coldot]');
    if (dot) { openColPop(dot); return; }
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
    // OSS-88: clicking empty rail space on a collapsed column expands it.
    // Fold/dot/title keep their own behavior above; any other click inside
    // a collapsed section (count badge, header padding, rail) expands.
    var collapsedSec = t.closest && t.closest('section.column.collapsed');
    if (collapsedSec) {
      var expCol = findCol(collapsedSec.dataset.col);
      if (!expCol) return;
      expCol.collapsed = false;
      saveCache(); render();
      api('PATCH', '/api/columns/' + collapsedSec.dataset.col, expCol).catch(function () {});
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
    // Click any note text to edit its RAW source directly (item 3; links
    // still navigate). The zero-layout "+ add text" affordance (item 1)
    // converts to an editable note in place with zero prior layout cost.
    var addText = t.closest && t.closest('.note-add[data-block]');
    if (addText) {
      var cardElA = addText.closest('[data-card]');
      var cardA = cardElA && findCard(cardElA.dataset.card);
      if (cardA) {
        var blkA = blockById(cardA, addText.dataset.block);
        if (!blkA) {
          blkA = { id: addText.dataset.block || '', type: 'note', content: '', position: cardA.blocks.length };
          cardA.blocks.push(blkA);
          saveCache(); render();
          focusNote(cardA.id, blkA.id, true);
        } else {
          var divA = document.createElement('div');
          divA.className = 'block note editing';
          divA.dataset.block = addText.dataset.block || '';
          divA.title = 'Click to edit';
          divA.textContent = blkA.content || '';
          addText.replaceWith(divA);
          startEdit(divA, true);
        }
      }
      return;
    }
    var note = t.closest && t.closest('.note[data-block]');
    if (note && !note.isContentEditable && !(t.tagName === 'A')) {
      // Empty-card placeholder also lands here via .note-add[data-block=""].
      var cardEl0 = note.closest('[data-card]');
      var card0 = cardEl0 && findCard(cardEl0.dataset.card);
      if (card0) {
        if (note.dataset.block === '' && !card0.blocks.length) {
          card0.blocks.push({ id: '', type: 'note', content: '', position: 0 });
          saveCache(); render();
          focusNote(card0.id, '', true);
        } else {
          editNoteRaw(note, blockById(card0, note.dataset.block), false);
        }
      }
      return;
    }
    // Todo text edits inline like notes (raw source, no markdown).
    // A row whose item is gone from the model is stale DOM (e.g. a
    // background sync replaced it): re-render instead of editing the
    // placeholder markup as if it were text.
    var todo = t.closest && t.closest('.todo-text[data-todo]');
    if (todo && !todo.isContentEditable) {
      var cardElT = todo.closest('[data-card]');
      var cardT = cardElT && findCard(cardElT.dataset.card);
      if (cardT) {
        var locT = todoLoc(cardT, todo.dataset.todolist, todo.dataset.todo);
        if (locT && locT.item) editTodoRaw(todo, locT.item, false);
        else render();
      }
      return;
    }
    // Todo item remove (× per row): drops that item only, keeping the
    // rest of the list; an emptied list drops its block too.
    var rmt = t.closest && t.closest('[data-rmtodo]');
    if (rmt) {
      var cardElM = rmt.closest('[data-card]');
      var cardM = cardElM && findCard(cardElM.dataset.card);
      if (cardM) {
        var locM = todoLoc(cardM, rmt.dataset.todolist, rmt.dataset.rmtodo);
        if (locM && locM.item) {
          removeTodoItem(cardM, locM);
          syncCard(cardM); render();
        }
      }
      return;
    }
    // Item 7: remove an image/link block via its × affordance (card PATCH).
    var rm = t.closest && t.closest('[data-rmblock]');
    if (rm) {
      var cardElR = rm.closest('[data-card]');
      var cardR = cardElR && findCard(cardElR.dataset.card);
      if (cardR) {
        cardR.blocks = cardR.blocks.filter(function (x) { return (x.id || '') !== (rm.dataset.rmblock || ''); });
        syncCard(cardR); render();
      }
      return;
    }
    // Item 2 (touch): first tap on a card reveals the overlay toolbar
    // instead of triggering an action; second tap acts.
    var cardTap = t.closest && t.closest('[data-card]');
    if (cardTap && !cardTap.classList.contains('showbar') &&
        window.matchMedia && matchMedia('(hover: none)').matches &&
        !(t.closest && (t.closest('[data-act]') || t.closest('a') || t.closest('button') || (t.isContentEditable)))) {
      cardTap.classList.add('showbar');
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
    } else if (act === 'addtodo') {
      // New checklist block (one block = one list); render-first like
      // addnote, then focus its first item. A second list needs another
      // explicit + todo — Enter never creates a block, only items.
      var nb = { id: tmpId(), type: 'todo', position: card.blocks.length, items: [{ id: tmpId(), content: '', checked: false }] };
      card.blocks.push(nb);
      saveCache(); render();
      focusTodoAt(card.id, countTodoItems(card) - 1, true);
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

  // Todo checkbox toggles one item via card PATCH without edit mode.
  boardEl.addEventListener('change', function (e) {
    var t = e.target;
    if (!t || !t.dataset || t.dataset.todotoggle === undefined) return;
    var cardEl = t.closest && t.closest('[data-card]');
    var card = cardEl && findCard(cardEl.dataset.card);
    if (!card) return;
    var loc = todoLoc(card, t.dataset.todolist, t.dataset.todotoggle);
    if (!loc || !loc.item || !!loc.item.checked === t.checked) return;
    loc.item.checked = t.checked;
    syncCard(card);
  });

  // Column color presets + custom color (round 3 item 4). The header
  // dot opens a small popover: 12 preset swatches plus a native custom
  // color input (live preview, commits on change). Persists via the
  // existing column PATCH path.
  var COL_PRESETS = ['#4c8dff', '#7aa5d8', '#00838f', '#4d7831', '#8fae85', '#697374',
    '#a96800', '#e8b96a', '#d8969c', '#c0392b', '#6b3d7d', '#37474f'];
  var colPop = null;
  function closeColPop() { if (colPop) { colPop.remove(); colPop = null; } }
  function commitColColor(id, color) {
    var col = findCol(id);
    if (!col) return;
    col.color = color;
    saveCache(); render();
    api('PATCH', '/api/columns/' + id, col).catch(function () {});
  }
  function openColPop(dot) {
    var id = dot.dataset.coldot;
    var col = findCol(id);
    if (!col) return;
    if (colPop && colPop.dataset.col === id) { closeColPop(); return; }
    closeColPop();
    var pop = document.createElement('div');
    pop.className = 'colpop';
    pop.dataset.col = id;
    pop.innerHTML = '<div class="colpop-grid" role="group" aria-label="Column color presets">' +
      COL_PRESETS.map(function (c) {
        return '<button data-preset="' + c + '" style="background:' + c + '" title="' + c + '" aria-label="Color ' + c + '"' +
          (col.color === c ? ' class="on"' : '') + '></button>';
      }).join('') + '</div><label class="colpop-custom">Custom <input type="color" data-custom value="' +
      esc(col.color || '#4c8dff') + '"></label>';
    document.body.appendChild(pop);
    var r = dot.getBoundingClientRect();
    pop.style.left = Math.max(8, Math.min(window.innerWidth - 194, r.left - 8)) + 'px';
    pop.style.top = (r.bottom + 6) + 'px';
    colPop = pop;
    pop.addEventListener('click', function (e) {
      var sw = e.target.closest && e.target.closest('[data-preset]');
      if (sw) { commitColColor(id, sw.dataset.preset); closeColPop(); }
    });
    var custom = pop.querySelector('[data-custom]');
    custom.addEventListener('input', function () {
      col.color = custom.value;
      dot.style.background = custom.value;
      var sec = boardEl.querySelector('[data-col="' + id + '"]');
      if (sec) sec.style.setProperty('--colc', custom.value);
      saveCache();
    });
    custom.addEventListener('change', function () { commitColColor(id, custom.value); closeColPop(); });
  }
  document.addEventListener('pointerdown', function (e) {
    if (colPop && !(e.target.closest && (e.target.closest('.colpop') || e.target.closest('[data-coldot]')))) closeColPop();
    // Round 4 item 6: dismiss the board overflow menu on outside press.
    if (boardMenuOpen && !(e.target.closest && e.target.closest('.boardmenu-wrap'))) { boardMenuOpen = false; render(); }
  }, true);
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') { closeColPop(); if (boardMenuOpen) { boardMenuOpen = false; render(); } }
  });

  // Commit inline edits on focus loss (blur doesn't bubble; focusout does).
  // Document-level: board tabs rename on double-click (round 4 item 2 —
  // no board-name repeat in the right zone), column titles and notes
  // live on the canvas — one listener covers all.
  document.addEventListener('focusout', function (e) {
    var t = e.target;
    if (!t || !t.isContentEditable) return;
    // OSS-64: a programmatic render() (e.g. Enter creating the next todo
    // row) detaches the focused node; its trailing blur must not commit
    // stale text and re-render again, which would steal the new row's
    // focus. Every render caller persists the model first, so a detached
    // node has nothing left to commit.
    if (!t.isConnected) return;
    t.contentEditable = 'false';
    t.classList.remove('editing');
    try { window.getSelection().removeAllRanges(); } catch (err) {}
    if (t.dataset && t.dataset.boardrename) {
      var bid = t.dataset.boardrename;
      delete t.dataset.boardrename;
      var title = t.innerText.trim();
      if (!title) { render(); return; }
      var known = null;
      state.boards.forEach(function (b) { if (b.id === bid) known = b; });
      if (!known || title === known.title) { render(); return; }
      known.title = title;
      var tr0 = state.trees[bid];
      if (tr0 && tr0.board) tr0.board.title = title;
      saveCache(); render();
      api('PATCH', '/api/boards/' + bid, { title: title }).then(function (b) {
        state.boards.forEach(function (x, i) { if (x.id === b.id) state.boards[i] = b; });
        var tr1 = state.trees[b.id];
        if (tr1) tr1.board = b;
        saveCache(); render();
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
    // Todo rows commit via commitTodo; Escape sets a cancel flag in the
    // keydown handler so uncommitted keystrokes are discarded instead.
    if (t.classList && t.classList.contains('todo-text')) {
      if (t.dataset.cancel) {
        delete t.dataset.cancel;
        var cardElC = t.closest && t.closest('[data-card]');
        var cardC = cardElC && findCard(cardElC.dataset.card);
        if (cardC) {
          var locC = todoLoc(cardC, t.dataset.todolist, t.dataset.todo);
          // Escape discards uncommitted keystrokes; an empty draft item
          // goes away, the rest of the list is untouched.
          if (locC && locC.item && !locC.item.content) {
            removeTodoItem(cardC, locC);
            saveCache(); syncCard(cardC);
          }
        }
        render();
        return;
      }
      commitTodo(t);
    }
  });
  // Enter commits an inline edit and exits edit mode; Shift+Enter,
  // Ctrl+Enter, AND Cmd/Meta+Enter all insert a newline (item 4 — all
  // three, not platform-specific). Applies to board/column titles/notes.
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
  document.addEventListener('keydown', function (e) {
    var t = e.target;
    if (!t || !t.isContentEditable) return;
    // Todo items: Enter commits text and creates the next item IN THE
    // SAME block (no new block). Enter on an empty item ends editing
    // without adding rows (an empty draft item is dropped, rest kept).
    // Escape ends editing, discarding uncommitted keystrokes.
    if (t.classList && t.classList.contains('todo-text')) {
      if (e.key === 'Escape') { e.preventDefault(); t.dataset.cancel = '1'; t.blur(); return; }
      if (e.key === 'Enter' && (e.shiftKey || e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        insertNewline(t);
        return;
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        var cardElE = t.closest && t.closest('[data-card]');
        var cardE = cardElE && findCard(cardElE.dataset.card);
        if (!cardE) { t.blur(); return; }
        var locE = todoLoc(cardE, t.dataset.todolist, t.dataset.todo);
        if (!locE || !locE.item) { t.blur(); render(); return; }
        var textE = t.innerText.replace(/\n+$/, '');
        if (!textE) {
          if (!locE.item.content) removeTodoItem(cardE, locE);
          saveCache(); syncCard(cardE); render();
          t.blur();
          return;
        }
        locE.item.content = textE;
        var ordE = todoOrdinal(cardE, locE.block.id, locE.item.id);
        locE.block.items.splice(locE.itemIndex + 1, 0, { id: tmpId(), content: '', checked: false });
        cardE.blocks.forEach(function (b, i) { b.position = i; });
        saveCache(); syncCard(cardE); render();
        focusTodoAt(cardE.id, ordE + 1, true);
        return;
      }
      return;
    }
    if (e.key === 'Escape') { e.preventDefault(); t.blur(); render(); }
    else if (e.key === 'Enter' && (e.shiftKey || e.ctrlKey || e.metaKey)) {
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
    // OSS-68: confirm-gated board delete (cancel = no state change,
    // no network DELETE).
    var del = e.target.closest && e.target.closest('[data-delboard]');
    if (del) { e.stopPropagation(); deleteBoard(del.dataset.delboard); return; }
    // Round 4 item 6: overflow menu toggle + reorder controls.
    var menu = e.target.closest && e.target.closest('[data-boardmenu]');
    if (menu) { boardMenuOpen = !boardMenuOpen; render(); return; }
    var up = e.target.closest && e.target.closest('[data-boardup]');
    if (up) { e.stopPropagation(); moveBoard(up.dataset.boardup, -1); return; }
    var down = e.target.closest && e.target.closest('[data-boarddown]');
    if (down) { e.stopPropagation(); moveBoard(down.dataset.boarddown, 1); return; }
    var b = e.target.closest && e.target.closest('[data-board]');
    if (b) {
      if (boardMenuOpen) boardMenuOpen = false;
      selectBoard(b.dataset.board);
    }
  });
  // Round 4 item 2: board rename lives on the tabs (no repeated board
  // name in the right zone) — double-click the active tab to edit inline.
  boardsEl.addEventListener('dblclick', function (e) {
    var b = e.target.closest && e.target.closest('[data-board]');
    if (!b) return;
    b.dataset.boardrename = b.dataset.board;
    startEdit(b, true);
  });

  // Round 4 item 2: the single navbar owns the right-zone controls —
  // expand-all/collapse-all first, then the theme switcher (background
  // swatches). Settings (language + import) lives in .actions.
  // No board title here.
  topbarCtl.addEventListener('click', function (e) {
    var t = e.target;
    // OSS-50: board share dialog (visibility + members, owner-managed).
    if (t.closest && t.closest('[data-share]')) { openShare(); return; }
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
  });

  // Legacy importer (round 3 item 1; OSS-57: lives in Settings): file
  // picker reads an OLD-style Loom v1 export JSON and POSTs it raw to
  // /api/import/v1, which creates new boards additively (never
  // overwrites/deletes). The new board is selected after revalidation
  // so the import is immediately visible.
  function doImportV1() {
    var input = document.createElement('input');
    input.type = 'file';
    input.accept = 'application/json,.json';
    input.onchange = function () {
      if (!input.files.length) return;
      var rd = new FileReader();
      rd.onload = function () {
        var raw = rd.result;
        try {
          var probe = JSON.parse(raw);
          if (!probe || !Array.isArray(probe.lists)) throw new Error('bad shape');
        } catch (err) {
          alert('Import failed: that file is not a Loom v1 export (missing "lists").');
          return;
        }
        fetch('/api/import/v1', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: raw })
          .then(function (r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
          .then(function (tree) {
            revalidate().then(function () {
              if (tree && tree.board) selectBoard(tree.board.id);
            });
          })
          .catch(function () { alert('Import failed on the server. Nothing was changed.'); });
      };
      rd.readAsText(input.files[0]);
    };
    input.click();
  }

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

  // ---- Auth UI (OSS-50): OIDC login/logout, UI settings, board share ----
  // Secrets stay server-side: settings GET/PUT never carry client_secret
  // to the browser (empty secret on save keeps the stored one).
  state.auth = { enabled: false, requireAuth: false, authenticated: false, user: null };
  var authBtn = document.getElementById('auth-btn');
  var settingsBtn = document.getElementById('settings-btn');
  function refreshAuth() {
    return api('GET', '/api/auth/status').then(function (s) {
      state.auth = { enabled: !!s.enabled, requireAuth: !!s.require_auth, authenticated: !!s.authenticated, user: s.user || null };
      paintAuth(); render();
    }).catch(function () {});
  }
  function paintAuth() {
    if (!authBtn || !settingsBtn) return;
    var a = state.auth;
    authBtn.style.display = a.enabled ? '' : 'none';
    authBtn.textContent = a.authenticated ? ('Logout' + (a.user && a.user.email ? ' (' + a.user.email + ')' : '')) : 'Login';
  }
  function openDialog(title, bodyHTML, onMount) {
    closeDialog();
    var ov = document.createElement('div');
    ov.className = 'dlg-ov';
    ov.innerHTML = '<div class="dlg" role="dialog" aria-label="' + esc(title) + '"><h3>' + esc(title) + '</h3><div class="dlg-body">' + bodyHTML + '</div><div class="dlg-foot"><button data-dlg-close>Close</button></div></div>';
    document.body.appendChild(ov);
    ov.addEventListener('click', function (e) {
      if (e.target === ov || (e.target.closest && e.target.closest('[data-dlg-close]'))) closeDialog();
    });
    if (onMount) onMount(ov);
    return ov;
  }
  function closeDialog() {
    var ov = document.querySelector('.dlg-ov');
    if (ov && ov.parentNode) ov.parentNode.removeChild(ov);
  }
  function dlgErr(ov, msg) {
    var el = ov.querySelector('.err');
    if (!el) {
      el = document.createElement('div');
      el.className = 'err';
      ov.querySelector('.dlg').appendChild(el);
    }
    el.textContent = msg;
  }
  function openSettings() {
    api('GET', '/api/auth/settings').then(function (s) {
      var ov = openDialog('Settings',
        '<label>OIDC issuer URL<input type="text" data-s="issuer" value="' + esc(s.issuer || '') + '" placeholder="https://accounts.example.com"></label>' +
        '<label>Client ID<input type="text" data-s="client_id" value="' + esc(s.client_id || '') + '"></label>' +
        '<label>Client secret (blank keeps stored' + (s.has_secret ? ' ✓' : '') + ')<input type="password" data-s="client_secret" value="" autocomplete="new-password"></label>' +
        '<label class="chk"><input type="checkbox" data-s="enabled"' + (s.enabled ? ' checked' : '') + '> Enable auth (login required, new boards private)</label>' +
        '<label class="chk"><input type="checkbox" data-s="require_auth"' + (s.require_auth ? ' checked' : '') + '> Require login even for public boards</label>' +
        '<div class="hint">Saved server-side in the database; disabling restores open access.</div>' +
        '<div style="margin-top:10px;text-align:right"><button class="primary" data-save>Save</button></div>' +
        '<hr><h4>Language</h4>' +
        '<div><button class="i18n-slot" disabled title="Language picker (coming soon)">EN</button> <span class="hint">More languages coming soon.</span></div>' +
        '<hr><h4>Import</h4>' +
        '<div><button id="import-v1" title="Import a Loom v1 export file (adds new boards, never deletes)">Import v1 file…</button> <span class="hint">Adds new boards, never deletes.</span></div>',
        function (ov) {
          var imp = ov.querySelector('#import-v1');
          if (imp) imp.addEventListener('click', doImportV1);
          ov.querySelector('[data-save]').addEventListener('click', function () {
            function val(k) { return ov.querySelector('[data-s="' + k + '"]').value; }
            function chk(k) { return ov.querySelector('[data-s="' + k + '"]').checked; }
            api('PUT', '/api/auth/settings', {
              issuer: val('issuer'), client_id: val('client_id'), client_secret: val('client_secret'),
              enabled: chk('enabled'), require_auth: chk('require_auth'),
            }).then(function () { closeDialog(); refreshAuth().then(revalidate); })
              .catch(function () { dlgErr(ov, 'Save failed (enabling needs issuer + client ID + secret).'); });
          });
        });
    }).catch(function () { alert('Settings need a login once auth is enabled.'); });
  }
  function openShare() {
    var tree = curTree();
    if (!tree || !state.auth.enabled) return;
    var bid = tree.board.id;
    api('GET', '/api/boards/' + bid + '/members').then(function (members) {
      var vis = tree.board.visibility || 'public';
      var rows = (members || []).map(function (m) {
        var who = m.email || m.name || m.user_id;
        return '<div class="share-row"><span title="' + esc(m.user_id) + '">' + esc(who) + '</span>' +
          '<select data-mrole="' + m.user_id + '"><option value="viewer"' + (m.role === 'viewer' ? ' selected' : '') + '>viewer</option>' +
          '<option value="editor"' + (m.role === 'editor' ? ' selected' : '') + '>editor</option></select>' +
          '<button data-mrm="' + m.user_id + '">Remove</button></div>';
      }).join('') || '<div class="hint">No members yet — only you (owner) can see this board.</div>';
      openDialog('Share: ' + tree.board.title,
        '<label>Visibility<select data-vis><option value="public"' + (vis === 'public' ? ' selected' : '') + '>public — anyone with the link</option>' +
        '<option value="private"' + (vis === 'private' ? ' selected' : '') + '>private — only you + members</option></select></label>' +
        '<div data-rows>' + rows + '</div>' +
        '<label>Invite by email (they log in once first)<input type="text" data-invite placeholder="teammate@example.com"></label>' +
        '<div style="margin-top:6px;text-align:right"><button data-invite-btn>Invite as viewer</button></div>',
        function (ov) {
          ov.querySelector('[data-vis]').addEventListener('change', function (e) {
            api('PATCH', '/api/boards/' + bid, { visibility: e.target.value }).then(function (b) {
              tree.board = b; saveCache(); render();
            }).catch(function () { dlgErr(ov, 'Visibility change failed.'); });
          });
          ov.querySelectorAll('[data-mrole]').forEach(function (sel) {
            sel.addEventListener('change', function () {
              api('PATCH', '/api/boards/' + bid + '/members/' + sel.dataset.mrole, { role: sel.value })
                .catch(function () { dlgErr(ov, 'Role change failed.'); });
            });
          });
          ov.querySelectorAll('[data-mrm]').forEach(function (btn) {
            btn.addEventListener('click', function () {
              api('DELETE', '/api/boards/' + bid + '/members/' + btn.dataset.mrm).then(function () {
                closeDialog(); openShare();
              }).catch(function () { dlgErr(ov, 'Remove failed.'); });
            });
          });
          ov.querySelector('[data-invite-btn]').addEventListener('click', function () {
            var email = ov.querySelector('[data-invite]').value.trim();
            if (!email) return;
            api('POST', '/api/boards/' + bid + '/members', { email: email, role: 'viewer' }).then(function () {
              closeDialog(); openShare();
            }).catch(function () { dlgErr(ov, 'Invite failed (that email has no login yet?).'); });
          });
        });
    }).catch(function () { alert('Share needs board-owner access.'); });
  }
  if (authBtn) authBtn.addEventListener('click', function () {
    if (state.auth.authenticated) {
      fetch('/api/auth/logout', { method: 'POST' }).then(function () { refreshAuth().then(revalidate); });
    } else {
      window.location.href = '/api/auth/login';
    }
  });
  if (settingsBtn) settingsBtn.addEventListener('click', openSettings);

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
  refreshAuth();
  if (window.requestIdleCallback) requestIdleCallback(revalidate);
  else setTimeout(revalidate, 0);
})();
