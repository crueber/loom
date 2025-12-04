// State
let currentUser = null;
let lists = [];
let bookmarks = {};

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

    // Initialize Sortable for lists
    new Sortable(container, {
        animation: 150,
        handle: '.list-header',
        scroll: true,
        scrollSensitivity: 100,
        scrollSpeed: 20,
        bubbleScroll: true,
        onEnd: handleListReorder
    });
}

function createListElement(list) {
    const div = document.createElement('div');
    div.className = `list-card ${list.collapsed ? 'collapsed' : ''}`;
    div.dataset.listId = list.id;

    const colorClass = getColorClass(list.color);

    div.innerHTML = `
        <div class="list-header ${colorClass}" data-list-id="${list.id}">
            <h3>${escapeHtml(list.title)}</h3>
            <div class="list-actions">
                <button class="list-action-btn edit-list" title="Edit">✏️</button>
                <button class="list-action-btn color-list" title="Change Color">🎨</button>
                <button class="list-action-btn delete-list" title="Delete">🗑️</button>
            </div>
        </div>
        <div class="bookmarks-container" data-list-id="${list.id}"></div>
        <button class="add-bookmark-btn secondary" data-list-id="${list.id}">+ Add Bookmark</button>
    `;

    // Event listeners
    div.querySelector('.list-header').addEventListener('click', (e) => {
        if (!e.target.closest('.list-actions')) {
            toggleListCollapse(list.id);
        }
    });

    div.querySelector('.edit-list').addEventListener('click', (e) => {
        e.stopPropagation();
        editListTitle(list.id);
    });

    div.querySelector('.color-list').addEventListener('click', (e) => {
        e.stopPropagation();
        changeListColor(list.id);
    });

    div.querySelector('.delete-list').addEventListener('click', (e) => {
        e.stopPropagation();
        deleteListConfirm(list.id);
    });

    div.querySelector('.add-bookmark-btn').addEventListener('click', () => {
        addBookmark(list.id);
    });

    // Load and render bookmarks
    loadBookmarks(list.id);

    return div;
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

    // Initialize Sortable for bookmarks
    new Sortable(container, {
        group: 'bookmarks',
        animation: 150,
        onEnd: handleBookmarkReorder
    });
}

function createBookmarkElement(bookmark) {
    const a = document.createElement('a');
    a.href = bookmark.url;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    a.className = 'bookmark-item';
    a.dataset.bookmarkId = bookmark.id;
    a.dataset.listId = bookmark.list_id;

    const faviconHtml = bookmark.favicon_url
        ? `<img src="${escapeHtml(bookmark.favicon_url)}" alt="">`
        : '<span class="bookmark-favicon-placeholder">📄</span>';

    a.innerHTML = `
        <div class="bookmark-favicon">${faviconHtml}</div>
        <div class="bookmark-content">
            <div class="bookmark-title">${escapeHtml(bookmark.title)}</div>
            <div class="bookmark-url">${escapeHtml(bookmark.url)}</div>
        </div>
        <div class="bookmark-actions">
            <button class="bookmark-action-btn edit-bookmark" title="Edit">✏️</button>
            <button class="bookmark-action-btn delete-bookmark" title="Delete">🗑️</button>
        </div>
    `;

    // Prevent default on action buttons
    a.querySelector('.edit-bookmark').addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        editBookmark(bookmark.id);
    });

    a.querySelector('.delete-bookmark').addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        deleteBookmarkConfirm(bookmark.id);
    });

    return a;
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

async function editListTitle(listId) {
    const list = lists.find(l => l.id === listId);
    if (!list) return;

    const newTitle = prompt('Enter new list title:', list.title);
    if (!newTitle || newTitle === list.title) return;

    try {
        await updateList(listId, { title: newTitle });
        list.title = newTitle;
        const headerEl = document.querySelector(`.list-header[data-list-id="${listId}"] h3`);
        headerEl.textContent = newTitle;
    } catch (error) {
        console.error('Failed to update list title:', error);
        alert('Failed to update list title');
    }
}

function showColorPicker(currentColor, callback) {
    const modal = document.getElementById('color-picker-modal');
    const grid = document.getElementById('color-picker-grid');

    grid.innerHTML = '';
    COLORS.forEach(color => {
        const div = document.createElement('div');
        div.className = `color-option ${color.class}`;
        if (color.value === currentColor) {
            div.classList.add('selected');
        }
        div.title = color.name;
        div.addEventListener('click', () => {
            callback(color);
            modal.close();
        });
        grid.appendChild(div);
    });

    modal.showModal();
}

async function changeListColor(listId) {
    const list = lists.find(l => l.id === listId);
    if (!list) return;

    showColorPicker(list.color, async (color) => {
        try {
            await updateList(listId, { color: color.value });
            list.color = color.value;

            const headerEl = document.querySelector(`.list-header[data-list-id="${listId}"]`);
            COLORS.forEach(c => headerEl.classList.remove(c.class));
            headerEl.classList.add(color.class);
        } catch (error) {
            console.error('Failed to update list color:', error);
            alert('Failed to update list color');
        }
    });
}

async function deleteListConfirm(listId) {
    const list = lists.find(l => l.id === listId);
    if (!list) return;

    if (!confirm(`Delete list "${list.title}"? This will also delete all bookmarks in this list.`)) {
        return;
    }

    try {
        await deleteList(listId);
        lists = lists.filter(l => l.id !== listId);
        delete bookmarks[listId];
        document.querySelector(`.list-card[data-list-id="${listId}"]`).remove();
    } catch (error) {
        console.error('Failed to delete list:', error);
        alert('Failed to delete list');
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

async function editBookmark(bookmarkId) {
    let bookmark = null;
    for (const listId in bookmarks) {
        bookmark = bookmarks[listId].find(b => b.id === bookmarkId);
        if (bookmark) break;
    }
    if (!bookmark) return;

    const newTitle = prompt('Enter new title:', bookmark.title);
    if (newTitle && newTitle !== bookmark.title) {
        try {
            await updateBookmark(bookmarkId, { title: newTitle });
            bookmark.title = newTitle;
            renderBookmarks(bookmark.list_id);
        } catch (error) {
            console.error('Failed to update bookmark:', error);
            alert('Failed to update bookmark');
        }
    }
}

async function deleteBookmarkConfirm(bookmarkId) {
    if (!confirm('Delete this bookmark?')) return;

    try {
        await deleteBookmark(bookmarkId);

        // Remove from local state
        for (const listId in bookmarks) {
            bookmarks[listId] = bookmarks[listId].filter(b => b.id !== bookmarkId);
            if (bookmarks[listId].some(b => b.id === bookmarkId)) {
                renderBookmarks(parseInt(listId));
                break;
            }
        }

        // Re-render the appropriate list
        for (const listId in bookmarks) {
            const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
            if (container && !container.querySelector(`[data-bookmark-id="${bookmarkId}"]`)) {
                continue;
            }
            renderBookmarks(parseInt(listId));
            break;
        }
    } catch (error) {
        console.error('Failed to delete bookmark:', error);
        alert('Failed to delete bookmark');
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

    const colorName = prompt(`Choose a color:\n${COLORS.map(c => c.name).join(', ')}`, 'Blue');
    const color = COLORS.find(c => c.name.toLowerCase() === colorName.toLowerCase()) || COLORS[0];

    try {
        const list = await createList(title, color.value);
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
        // Only activate on container background or bookmarks-container whitespace
        const target = e.target;
        const isValidTarget = target === container ||
                             target.classList.contains('bookmarks-container') ||
                             target.classList.contains('lists-wrapper');

        if (!isValidTarget) return;

        isDown = true;
        container.style.cursor = 'grabbing';
        startX = e.pageX - container.offsetLeft;
        scrollLeft = container.scrollLeft;
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
        e.preventDefault();
        const x = e.pageX - container.offsetLeft;
        const walk = (x - startX) * 2;
        container.scrollLeft = scrollLeft - walk;
    });

    // Set initial cursor
    container.style.cursor = 'grab';
}

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
