// State
let currentUser = null;
let lists = [];
let bookmarks = {};

// Touch device detection
const isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
if (isTouchDevice) {
    document.body.classList.add('touch-device');
}

// Track currently flipped card
let currentlyFlippedCard = null;

// Track Sortable instances
let listsSortable = null;
let bookmarkSortables = {};

// Color palette - Darker, more readable colors
const COLORS = [
    { name: 'Blue', value: '#3D6D95', class: 'color-blue' },
    { name: 'Green', value: '#4D7831', class: 'color-green' },
    { name: 'Orange', value: '#B85720', class: 'color-orange' },
    { name: 'Red', value: '#A43529', class: 'color-red' },
    { name: 'Purple', value: '#6B3D7D', class: 'color-purple' },
    { name: 'Pink', value: '#924F7D', class: 'color-pink' },
    { name: 'Teal', value: '#358178', class: 'color-teal' },
    { name: 'Gray', value: '#697374', class: 'color-gray' }
];

// API Helper Functions
async function apiCall(endpoint, options = {}) {
    const response = await fetch(`/api${endpoint}`, {
        ...options,
        headers: {
            'Content-Type': 'application/json',
            ...options.headers
        }
    });

    if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Request failed');
    }

    if (response.status === 204) {
        return null;
    }

    return response.json();
}

// Authentication
async function login(username, password) {
    return apiCall('/login', {
        method: 'POST',
        body: JSON.stringify({ username, password })
    });
}

async function logout() {
    return apiCall('/logout', { method: 'POST' });
}

async function getCurrentUser() {
    return apiCall('/user');
}

// Lists API
async function getLists() {
    return apiCall('/lists');
}

async function createList(title, color) {
    return apiCall('/lists', {
        method: 'POST',
        body: JSON.stringify({ title, color })
    });
}

async function updateList(id, data) {
    return apiCall(`/lists/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data)
    });
}

async function deleteList(id) {
    return apiCall(`/lists/${id}`, { method: 'DELETE' });
}

async function reorderLists(listsData) {
    return apiCall('/lists/reorder', {
        method: 'PUT',
        body: JSON.stringify({ lists: listsData })
    });
}

// Bookmarks API
async function getBookmarks(listId) {
    return apiCall(`/lists/${listId}/bookmarks`);
}

async function createBookmark(listId, title, url) {
    return apiCall('/bookmarks', {
        method: 'POST',
        body: JSON.stringify({ list_id: listId, title, url })
    });
}

async function updateBookmark(id, data) {
    return apiCall(`/bookmarks/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data)
    });
}

async function deleteBookmark(id) {
    return apiCall(`/bookmarks/${id}`, { method: 'DELETE' });
}

async function reorderBookmarks(bookmarksData) {
    return apiCall('/bookmarks/reorder', {
        method: 'PUT',
        body: JSON.stringify({ bookmarks: bookmarksData })
    });
}

// Export/Import API
async function exportData() {
    const response = await fetch('/api/export');
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `home-links-export-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
}

async function importData(data, mode) {
    return apiCall('/import', {
        method: 'POST',
        body: JSON.stringify({ data, mode })
    });
}

// UI Helper Functions
function showScreen(screenId) {
    document.querySelectorAll('[id$="-screen"]').forEach(screen => {
        screen.style.display = 'none';
    });
    document.getElementById(screenId).style.display = 'block';
}

function showError(elementId, message) {
    const errorEl = document.getElementById(elementId);
    errorEl.textContent = message;
    errorEl.style.display = 'block';
}

function hideError(elementId) {
    const errorEl = document.getElementById(elementId);
    errorEl.style.display = 'none';
}

function getColorClass(colorValue) {
    const color = COLORS.find(c => c.value === colorValue);
    return color ? color.class : 'color-blue';
}

// Render Functions
function renderLists() {
    const container = document.getElementById('lists-container');

    // Remove only list cards, keep the add-list-container
    const listCards = container.querySelectorAll('.list-card');
    listCards.forEach(card => card.remove());

    // Insert lists before the add-list-container
    const addListContainer = document.getElementById('add-list-container');
    lists.forEach(list => {
        const listEl = createListElement(list);
        container.insertBefore(listEl, addListContainer);
    });

    // Initialize Sortable for lists (destroy old instance if exists)
    if (listsSortable) {
        listsSortable.destroy();
    }
    listsSortable = new Sortable(container, {
        animation: 150,
        handle: '.list-header',
        filter: '[data-flipped="true"]',
        scroll: true,
        scrollSensitivity: 100,
        scrollSpeed: 20,
        bubbleScroll: true,
        delay: 200,
        delayOnTouchOnly: true,
        onEnd: handleListReorder
    });
}

function createListElement(list) {
    const div = document.createElement('div');
    div.className = `list-card ${list.collapsed ? 'collapsed' : ''}`;
    div.dataset.listId = list.id;
    div.dataset.flipped = 'false';

    const colorClass = getColorClass(list.color);

    div.innerHTML = `
        <div class="list-card-inner">
            <div class="list-card-front">
                <div class="list-header ${colorClass}" data-list-id="${list.id}">
                    <h3>${escapeHtml(list.title)}</h3>
                    <div class="list-actions">
                        <button class="list-action-btn config-list" title="Configure">⚙️</button>
                    </div>
                </div>
                <div class="bookmarks-container" data-list-id="${list.id}"></div>
                <button class="add-bookmark-btn secondary" data-list-id="${list.id}">+ Add Bookmark</button>
            </div>
            <div class="list-card-back">
                <div class="list-config-panel">
                    <div class="list-config-header">
                        <h4>Configure List</h4>
                        <button class="config-close-btn" title="Close">×</button>
                    </div>
                    <div class="config-form-group">
                        <label>List Name</label>
                        <input type="text" class="config-list-title" value="${escapeHtml(list.title)}">
                    </div>
                    <div class="config-form-group">
                        <label>Color</label>
                        <div class="config-color-grid">
                            ${COLORS.map(color => `
                                <div class="config-color-option ${color.class} ${color.value === list.color ? 'selected' : ''}"
                                     data-color="${color.value}"
                                     title="${color.name}"></div>
                            `).join('')}
                        </div>
                    </div>
                    <div class="config-actions">
                        <button class="config-delete-btn secondary">Delete List</button>
                        <button class="config-cancel-btn secondary">Cancel</button>
                        <button class="config-save-btn">Save</button>
                    </div>
                </div>
            </div>
        </div>
    `;

    // Event listeners - List header collapse/expand
    div.querySelector('.list-header').addEventListener('click', (e) => {
        // Only toggle collapse when card is not flipped
        if (div.dataset.flipped === 'true') return;
        if (!e.target.closest('.list-actions')) {
            toggleListCollapse(list.id);
        }
    });

    // Gear icon - flip to back
    div.querySelector('.config-list').addEventListener('click', (e) => {
        e.stopPropagation();
        flipToList(list.id);
    });

    // Close button - flip to front
    div.querySelector('.config-close-btn').addEventListener('click', () => {
        closeFlippedCard();
    });

    // Cancel button - flip to front
    div.querySelector('.config-cancel-btn').addEventListener('click', () => {
        closeFlippedCard();
    });

    // Color selection
    div.querySelectorAll('.config-color-option').forEach(option => {
        option.addEventListener('click', () => {
            div.querySelectorAll('.config-color-option').forEach(o => o.classList.remove('selected'));
            option.classList.add('selected');
        });
    });

    // Save button
    div.querySelector('.config-save-btn').addEventListener('click', () => {
        saveListConfig(list.id);
    });

    // Delete button
    div.querySelector('.config-delete-btn').addEventListener('click', () => {
        deleteListFromConfig(list.id);
    });

    // Enter key to save
    div.querySelector('.config-list-title').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            saveListConfig(list.id);
        }
    });

    // Add bookmark button
    div.querySelector('.add-bookmark-btn').addEventListener('click', () => {
        addBookmark(list.id);
    });

    // Load and render bookmarks
    loadBookmarks(list.id);

    return div;
}

// List card flip helper functions
function flipToList(listId) {
    // Close currently flipped card if it's different
    if (currentlyFlippedCard && (currentlyFlippedCard.type !== 'list' || currentlyFlippedCard.id !== listId)) {
        closeFlippedCard();
    }

    const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
    if (!card) return;

    card.dataset.flipped = 'true';
    currentlyFlippedCard = { type: 'list', id: listId };

    // Disable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", true);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", true);
    });

    // Reset form to current values
    const list = lists.find(l => l.id === listId);
    if (list) {
        const input = card.querySelector('.config-list-title');
        input.value = list.title;

        // Reset color selection
        card.querySelectorAll('.config-color-option').forEach(option => {
            option.classList.toggle('selected', option.dataset.color === list.color);
        });

        // Focus first input after animation
        setTimeout(() => input.focus(), 600);
    }
}

async function saveListConfig(listId) {
    const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
    const list = lists.find(l => l.id === listId);
    if (!card || !list) return;

    const newTitle = card.querySelector('.config-list-title').value.trim();
    const selectedColor = card.querySelector('.config-color-option.selected')?.dataset.color;

    if (!newTitle) {
        alert('List title cannot be empty');
        return;
    }

    try {
        const updates = {};
        if (newTitle !== list.title) updates.title = newTitle;
        if (selectedColor && selectedColor !== list.color) updates.color = selectedColor;

        if (Object.keys(updates).length > 0) {
            await updateList(listId, updates);

            // Update local state
            if (updates.title) list.title = updates.title;
            if (updates.color) list.color = updates.color;

            // Update front of card
            if (updates.title) {
                card.querySelector('.list-header h3').textContent = updates.title;
            }
            if (updates.color) {
                const headerEl = card.querySelector('.list-header');
                COLORS.forEach(c => headerEl.classList.remove(c.class));
                const colorClass = getColorClass(updates.color);
                headerEl.classList.add(colorClass);
            }
        }

        closeFlippedCard();
    } catch (error) {
        console.error('Failed to save list configuration:', error);
        alert('Failed to save changes');
    }
}

async function deleteListFromConfig(listId) {
    const list = lists.find(l => l.id === listId);
    if (!list) return;

    const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
    const panel = card.querySelector('.list-config-panel');

    // Store original HTML
    const originalHTML = panel.innerHTML;

    // Show confirmation UI
    panel.innerHTML = `
        <div class="list-config-header">
            <h4>Delete List?</h4>
        </div>
        <p style="margin: 1rem 0;">Delete "${escapeHtml(list.title)}"? This will also delete all bookmarks in this list.</p>
        <div class="config-actions">
            <button class="confirm-delete-cancel-btn secondary">Cancel</button>
            <button class="confirm-delete-btn config-delete-btn">Delete</button>
        </div>
    `;

    // Cancel confirmation
    panel.querySelector('.confirm-delete-cancel-btn').addEventListener('click', () => {
        panel.innerHTML = originalHTML;
        closeFlippedCard();
    });

    // Confirm delete
    panel.querySelector('.confirm-delete-btn').addEventListener('click', async () => {
        try {
            await deleteList(listId);
            lists = lists.filter(l => l.id !== listId);
            delete bookmarks[listId];
            currentlyFlippedCard = null;
            card.remove();
        } catch (error) {
            console.error('Failed to delete list:', error);
            alert('Failed to delete list');
            panel.innerHTML = originalHTML;
        }
    });
}

function closeFlippedCard() {
    if (!currentlyFlippedCard) return;

    if (currentlyFlippedCard.type === 'list') {
        const card = document.querySelector(`.list-card[data-list-id="${currentlyFlippedCard.id}"]`);
        if (card) {
            card.dataset.flipped = 'false';
        }
    } else if (currentlyFlippedCard.type === 'bookmark') {
        const bookmark = document.querySelector(`.bookmark-item[data-bookmark-id="${currentlyFlippedCard.id}"]`);
        if (bookmark) {
            bookmark.dataset.flipped = 'false';
        }
    }

    currentlyFlippedCard = null;

    // Re-enable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", false);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", false);
    });
}

// Bookmark card flip helper functions
function flipToBookmark(bookmarkId) {
    // Close currently flipped card if it's different
    if (currentlyFlippedCard && (currentlyFlippedCard.type !== 'bookmark' || currentlyFlippedCard.id !== bookmarkId)) {
        closeFlippedCard();
    }

    const bookmarkEl = document.querySelector(`.bookmark-item[data-bookmark-id="${bookmarkId}"]`);
    if (!bookmarkEl) return;

    bookmarkEl.dataset.flipped = 'true';
    currentlyFlippedCard = { type: 'bookmark', id: bookmarkId };

    // Disable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", true);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", true);
    });

    // Find bookmark
    let bookmark = null;
    for (const listId in bookmarks) {
        bookmark = bookmarks[listId].find(b => b.id === bookmarkId);
        if (bookmark) break;
    }

    if (bookmark) {
        bookmarkEl.querySelector('.config-bookmark-title').value = bookmark.title;
        bookmarkEl.querySelector('.config-bookmark-url').value = bookmark.url;

        // Focus first input after animation
        setTimeout(() => bookmarkEl.querySelector('.config-bookmark-title').focus(), 300);
    }
}

async function saveBookmarkConfig(bookmarkId) {
    const bookmarkEl = document.querySelector(`.bookmark-item[data-bookmark-id="${bookmarkId}"]`);
    if (!bookmarkEl) return;

    // Find bookmark
    let bookmark = null;
    for (const listId in bookmarks) {
        bookmark = bookmarks[listId].find(b => b.id === bookmarkId);
        if (bookmark) break;
    }
    if (!bookmark) return;

    const newTitle = bookmarkEl.querySelector('.config-bookmark-title').value.trim();
    const newUrl = bookmarkEl.querySelector('.config-bookmark-url').value.trim();

    if (!newTitle || !newUrl) {
        alert('Title and URL cannot be empty');
        return;
    }

    try {
        const updates = {};
        if (newTitle !== bookmark.title) updates.title = newTitle;
        if (newUrl !== bookmark.url) updates.url = newUrl;

        if (Object.keys(updates).length > 0) {
            await updateBookmark(bookmarkId, updates);

            // Update local state
            if (updates.title) bookmark.title = updates.title;
            if (updates.url) bookmark.url = updates.url;

            // Re-render to update front
            renderBookmarks(bookmark.list_id);
        } else {
            closeFlippedCard();
        }
    } catch (error) {
        console.error('Failed to save bookmark:', error);
        alert('Failed to save changes');
    }
}

async function deleteBookmarkFromConfig(bookmarkId) {
    const bookmarkEl = document.querySelector(`.bookmark-item[data-bookmark-id="${bookmarkId}"]`);
    if (!bookmarkEl) return;

    // Find bookmark
    let bookmark = null;
    for (const listId in bookmarks) {
        bookmark = bookmarks[listId].find(b => b.id === bookmarkId);
        if (bookmark) break;
    }
    if (!bookmark) return;

    const panel = bookmarkEl.querySelector('.bookmark-config-panel');
    const originalHTML = panel.innerHTML;

    // Show confirmation
    panel.innerHTML = `
        <div class="bookmark-config-header">
            <h5>Delete Bookmark?</h5>
        </div>
        <p style="margin: 0.5rem 0; font-size: 0.875rem;">Delete "${escapeHtml(bookmark.title)}"?</p>
        <div class="bookmark-config-actions">
            <button class="bookmark-confirm-cancel-btn secondary">Cancel</button>
            <button class="bookmark-confirm-delete-btn config-delete-btn">Delete</button>
        </div>
    `;

    panel.querySelector('.bookmark-confirm-cancel-btn').addEventListener('click', (e) => {
        e.preventDefault();
        panel.innerHTML = originalHTML;
        closeFlippedCard();
    });

    panel.querySelector('.bookmark-confirm-delete-btn').addEventListener('click', async (e) => {
        e.preventDefault();
        try {
            await deleteBookmark(bookmarkId);

            // Remove from local state
            for (const listId in bookmarks) {
                bookmarks[listId] = bookmarks[listId].filter(b => b.id !== bookmarkId);
            }

            // Clear flipped card state
            currentlyFlippedCard = null;

            // Remove element
            bookmarkEl.remove();
        } catch (error) {
            console.error('Failed to delete bookmark:', error);
            alert('Failed to delete bookmark');
        }
    });
}

function renderBookmarks(listId) {
    const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
    if (!container) return;

    const listBookmarks = bookmarks[listId] || [];
    container.innerHTML = '';

    listBookmarks.forEach(bookmark => {
        const bookmarkEl = createBookmarkElement(bookmark);
        container.appendChild(bookmarkEl);
    });

    // Initialize Sortable for bookmarks (destroy old instance if exists)
    if (bookmarkSortables[listId]) {
        bookmarkSortables[listId].destroy();
    }
    bookmarkSortables[listId] = new Sortable(container, {
        group: 'bookmarks',
        animation: 150,
        handle: '.bookmark-card-front',  // Only allow dragging from the front of the card
        filter: '[data-flipped="true"]',
        delay: 200,
        delayOnTouchOnly: true,
        dragClass: 'sortable-drag',
        onStart: function(evt) {
            // If any card is flipped, cancel the drag
            if (currentlyFlippedCard) {
                evt.item.dispatchEvent(new Event('mouseup'));
                return false;
            }
        },
        onEnd: handleBookmarkReorder,
        onMove: function(evt) {
            // Don't allow dragging if the item is flipped
            return evt.related.dataset.flipped !== 'true';
        }
    });
}

function createBookmarkElement(bookmark) {
    const wrapper = document.createElement('div');
    wrapper.className = 'bookmark-item';
    wrapper.dataset.bookmarkId = bookmark.id;
    wrapper.dataset.listId = bookmark.list_id;
    wrapper.dataset.flipped = 'false';

    const faviconHtml = bookmark.favicon_url
        ? `<img src="${escapeHtml(bookmark.favicon_url)}" alt="">`
        : '<span class="bookmark-favicon-placeholder">📄</span>';

    wrapper.innerHTML = `
        <div class="bookmark-card-inner">
            <a href="${escapeHtml(bookmark.url)}" target="_blank" rel="noopener noreferrer" class="bookmark-card-front">
                <div class="bookmark-favicon">${faviconHtml}</div>
                <div class="bookmark-content">
                    <div class="bookmark-title">${escapeHtml(bookmark.title)}</div>
                    <div class="bookmark-url">${escapeHtml(bookmark.url)}</div>
                </div>
                <div class="bookmark-actions">
                    <button class="bookmark-action-btn config-bookmark" title="Configure">⚙️</button>
                </div>
            </a>
            <div class="bookmark-card-back">
                <div class="bookmark-config-panel">
                    <div class="bookmark-config-header">
                        <h5>Edit Bookmark</h5>
                        <button class="bookmark-config-close-btn" title="Close">×</button>
                    </div>
                    <input type="text" class="config-bookmark-title" placeholder="Title" value="${escapeHtml(bookmark.title)}">
                    <input type="url" class="config-bookmark-url" placeholder="URL" value="${escapeHtml(bookmark.url)}">
                    <div class="bookmark-config-actions">
                        <button class="bookmark-config-delete-btn secondary">Delete</button>
                        <button class="bookmark-config-cancel-btn secondary">Cancel</button>
                        <button class="bookmark-config-save-btn">Save</button>
                    </div>
                </div>
            </div>
        </div>
    `;

    // Gear icon - flip to back
    wrapper.querySelector('.config-bookmark').addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        flipToBookmark(bookmark.id);
    });

    // Close button
    wrapper.querySelector('.bookmark-config-close-btn').addEventListener('click', (e) => {
        e.preventDefault();
        closeFlippedCard();
    });

    // Cancel button
    wrapper.querySelector('.bookmark-config-cancel-btn').addEventListener('click', (e) => {
        e.preventDefault();
        closeFlippedCard();
    });

    // Save button
    wrapper.querySelector('.bookmark-config-save-btn').addEventListener('click', (e) => {
        e.preventDefault();
        saveBookmarkConfig(bookmark.id);
    });

    // Delete button
    wrapper.querySelector('.bookmark-config-delete-btn').addEventListener('click', (e) => {
        e.preventDefault();
        deleteBookmarkFromConfig(bookmark.id);
    });

    // Enter key to save on either input
    const titleInput = wrapper.querySelector('.config-bookmark-title');
    const urlInput = wrapper.querySelector('.config-bookmark-url');

    titleInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            saveBookmarkConfig(bookmark.id);
        }
    });

    urlInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            saveBookmarkConfig(bookmark.id);
        }
    });

    return wrapper;
}

// Data Loading
async function loadBookmarks(listId) {
    try {
        const data = await getBookmarks(listId);
        bookmarks[listId] = data;
        renderBookmarks(listId);
    } catch (error) {
        console.error('Failed to load bookmarks:', error);
    }
}

async function loadData() {
    try {
        lists = await getLists();
        renderLists();
    } catch (error) {
        console.error('Failed to load lists:', error);
        alert('Failed to load data');
    }
}

// List Actions
async function toggleListCollapse(listId) {
    const list = lists.find(l => l.id === listId);
    if (!list) return;

    list.collapsed = !list.collapsed;

    try {
        await updateList(listId, { collapsed: list.collapsed });
        const listEl = document.querySelector(`.list-card[data-list-id="${listId}"]`);
        listEl.classList.toggle('collapsed', list.collapsed);
    } catch (error) {
        console.error('Failed to toggle list:', error);
        list.collapsed = !list.collapsed;
    }
}

async function handleListReorder(evt) {
    const listElements = document.querySelectorAll('.list-card');
    const reorderedLists = Array.from(listElements).map((el, index) => ({
        id: parseInt(el.dataset.listId),
        position: index
    }));

    try {
        await reorderLists(reorderedLists);

        // Update local state
        lists.forEach(list => {
            const item = reorderedLists.find(r => r.id === list.id);
            if (item) list.position = item.position;
        });
        lists.sort((a, b) => a.position - b.position);
    } catch (error) {
        console.error('Failed to reorder lists:', error);
        // Reload on failure
        loadData();
    }
}

// Bookmark Actions
async function addBookmark(listId) {
    const url = prompt('Enter bookmark URL:');
    if (!url) return;

    const title = prompt('Enter bookmark title:');
    if (!title) return;

    try {
        const bookmark = await createBookmark(listId, title, url);
        if (!bookmarks[listId]) bookmarks[listId] = [];
        bookmarks[listId].push(bookmark);
        renderBookmarks(listId);
    } catch (error) {
        console.error('Failed to create bookmark:', error);
        alert('Failed to create bookmark: ' + error.message);
    }
}

async function handleBookmarkReorder(evt) {
    const { from, to, item } = evt;
    const bookmarkId = parseInt(item.dataset.bookmarkId);
    const newListId = parseInt(to.dataset.listId);

    const bookmarkElements = to.querySelectorAll('.bookmark-item');
    const reorderedBookmarks = Array.from(bookmarkElements).map((el, index) => ({
        id: parseInt(el.dataset.bookmarkId),
        position: index,
        list_id: newListId
    }));

    try {
        await reorderBookmarks(reorderedBookmarks);

        // Update local state
        for (const listId in bookmarks) {
            const bookmark = bookmarks[listId].find(b => b.id === bookmarkId);
            if (bookmark) {
                bookmarks[listId] = bookmarks[listId].filter(b => b.id !== bookmarkId);
                bookmark.list_id = newListId;
                bookmark.position = reorderedBookmarks.find(r => r.id === bookmarkId).position;

                if (!bookmarks[newListId]) bookmarks[newListId] = [];
                bookmarks[newListId].push(bookmark);
                break;
            }
        }

        // Update all affected lists
        const affectedListIds = new Set([...reorderedBookmarks.map(r => r.list_id)]);
        affectedListIds.forEach(listId => {
            bookmarks[listId].sort((a, b) => a.position - b.position);
        });
    } catch (error) {
        console.error('Failed to reorder bookmarks:', error);
        // Reload on failure
        loadData();
    }
}

// Utility Functions
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Event Handlers
document.getElementById('login-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    hideError('login-error');

    const username = document.getElementById('login-username').value;
    const password = document.getElementById('login-password').value;

    try {
        currentUser = await login(username, password);
        showScreen('app-screen');
        document.getElementById('user-name').textContent = currentUser.username;
        await loadData();
    } catch (error) {
        showError('login-error', error.message);
    }
});

document.getElementById('logout-btn').addEventListener('click', async () => {
    try {
        await logout();
        currentUser = null;
        lists = [];
        bookmarks = {};
        showScreen('login-screen');
        document.getElementById('login-form').reset();
    } catch (error) {
        console.error('Logout failed:', error);
    }
});

document.getElementById('add-list-btn').addEventListener('click', async () => {
    const title = prompt('Enter list title:');
    if (!title) return;

    // Use default color (first in array - Blue)
    const defaultColor = COLORS[0].value;

    try {
        const list = await createList(title, defaultColor);
        lists.push(list);
        renderLists();
    } catch (error) {
        console.error('Failed to create list:', error);
        alert('Failed to create list');
    }
});

document.getElementById('export-btn').addEventListener('click', async () => {
    try {
        await exportData();
    } catch (error) {
        console.error('Export failed:', error);
        alert('Export failed');
    }
});

document.getElementById('import-btn').addEventListener('click', () => {
    document.getElementById('import-modal').showModal();
});

document.getElementById('close-color-picker').addEventListener('click', () => {
    document.getElementById('color-picker-modal').close();
});

document.getElementById('close-import-modal').addEventListener('click', () => {
    document.getElementById('import-modal').close();
});

document.getElementById('cancel-import-btn').addEventListener('click', () => {
    document.getElementById('import-modal').close();
});

document.getElementById('confirm-import-btn').addEventListener('click', async () => {
    hideError('import-error');

    const fileInput = document.getElementById('import-file');
    const mode = document.getElementById('import-mode').value;

    if (!fileInput.files[0]) {
        showError('import-error', 'Please select a file');
        return;
    }

    try {
        const fileContent = await fileInput.files[0].text();
        const data = JSON.parse(fileContent);

        await importData(data, mode);
        document.getElementById('import-modal').close();
        document.getElementById('import-form').reset();
        await loadData();
        alert('Import successful!');
    } catch (error) {
        console.error('Import failed:', error);
        showError('import-error', error.message);
    }
});

// Initialize horizontal drag scrolling
function initializeHorizontalDragScroll() {
    const container = document.getElementById('lists-container');
    let isDown = false;
    let startX;
    let scrollLeft;

    container.addEventListener('mousedown', (e) => {
        // Don't activate if any card is flipped
        if (currentlyFlippedCard) {
            return;
        }

        // Don't activate on input elements, buttons, or other interactive elements
        // Check both target and closest parent to handle labels
        let element = e.target;
        while (element && element !== container) {
            if (element.tagName === 'INPUT' ||
                element.tagName === 'BUTTON' ||
                element.tagName === 'A' ||
                element.tagName === 'TEXTAREA' ||
                element.tagName === 'SELECT' ||
                element.tagName === 'LABEL') {
                return;
            }
            element = element.parentElement;
        }

        // Only activate on container background or bookmarks-container whitespace
        const isValidTarget = e.target === container ||
                             e.target.classList.contains('bookmarks-container') ||
                             e.target.classList.contains('lists-wrapper');

        if (!isValidTarget) return;

        isDown = true;
        container.style.cursor = 'grabbing';
        startX = e.pageX - container.offsetLeft;
        scrollLeft = container.scrollLeft;
        e.preventDefault(); // Prevent text selection when dragging valid targets
    });

    container.addEventListener('mouseleave', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mouseup', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mousemove', (e) => {
        if (!isDown) return;

        // Don't prevent default on interactive elements
        const target = e.target;
        if (target.tagName !== 'INPUT' &&
            target.tagName !== 'BUTTON' &&
            target.tagName !== 'A' &&
            target.tagName !== 'TEXTAREA' &&
            target.tagName !== 'SELECT') {
            e.preventDefault();
        }

        const x = e.pageX - container.offsetLeft;
        const walk = (x - startX) * 2;
        container.scrollLeft = scrollLeft - walk;
    });

    // Set initial cursor
    container.style.cursor = 'grab';
}

// Keyboard event listeners
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && currentlyFlippedCard) {
        closeFlippedCard();
    }
});

// Initialize
(async () => {
    try {
        currentUser = await getCurrentUser();
        showScreen('app-screen');
        document.getElementById('user-name').textContent = currentUser.username;
        await loadData();
        initializeHorizontalDragScroll();
    } catch (error) {
        showScreen('login-screen');
    }
})();
