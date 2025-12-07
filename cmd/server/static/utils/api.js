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

async function createList(title, color, boardId) {
    return apiCall('/lists', {
        method: 'POST',
        body: JSON.stringify({ title, color, board_id: boardId })
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

// Utility Functions
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
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
