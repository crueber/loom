// API Helper Functions
async function _apiCall(endpoint, options = {}) {
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

    return response;
}

async function apiCall(endpoint, options = {}) {    
    const response = await _apiCall(endpoint, options);

    if (response.status === 204) {
        return null;
    }

    return response.json();
}

async function apiCallExpectEmpty(endpoint, options = {}) {
    const response = await _apiCall(endpoint, options);

    return null;
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
    return apiCallExpectEmpty(`/lists/${id}`, { method: 'DELETE' });
}

async function reorderLists(listsData) {
    return apiCall('/lists/reorder', {
        method: 'PUT',
        body: JSON.stringify({ lists: listsData })
    });
}

async function copyOrMoveList(listId, targetBoardId, copy) {
    return apiCall(`/lists/${listId}/copy-or-move`, {
        method: 'POST',
        body: JSON.stringify({ target_board_id: targetBoardId, copy })
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

// Items API (unified bookmarks and notes)
async function getItems(listId) {
    return apiCall(`/lists/${listId}/items`);
}

async function createItem(listId, type, data) {
    return apiCall('/items', {
        method: 'POST',
        body: JSON.stringify({ list_id: listId, type, ...data })
    });
}

async function updateItem(id, data) {
    return apiCall(`/items/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data)
    });
}

async function deleteItem(id) {
    return apiCall(`/items/${id}`, { method: 'DELETE' });
}

async function reorderItems(itemsData) {
    return apiCall('/items/reorder', {
        method: 'PUT',
        body: JSON.stringify({ items: itemsData })
    });
}

// Boards API
async function getBoards() {
    return apiCall('/boards');
}

async function getBoard(boardId) {
    return apiCall(`/boards/${boardId}`);
}

async function getBoardData(boardId) {
    return apiCall(`/boards/${boardId}/data`);
}

async function createBoard(title) {
    return apiCall('/boards', {
        method: 'POST',
        body: JSON.stringify({ title })
    });
}

async function updateBoard(boardId, title) {
    return apiCallExpectEmpty(`/boards/${boardId}`, {
        method: 'PUT',
        body: JSON.stringify({ title })
    });
}

async function deleteBoard(boardId) {
    return apiCallExpectEmpty(`/boards/${boardId}`, { method: 'DELETE' });
}

// Data API
async function getAllData() {
    return apiCall('/data');
}

// Export/Import API
async function exportData() {
    const response = await fetch('/api/export');
    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `loom-export-${new Date().toISOString().split('T')[0]}.json`;
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

/**
 * Debounces a function call, ensuring it's only called once after a delay
 * Useful for preventing excessive API calls during rapid user interactions
 *
 * @param {Function} func - Function to debounce
 * @param {number} wait - Delay in milliseconds
 * @returns {Function} Debounced function
 *
 * @example
 * const debouncedSave = debounce(() => saveData(), 300);
 * element.addEventListener('input', debouncedSave);
 */
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

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

// Event Dispatcher Facade
/**
 * Dispatches a CustomEvent on the document
 * Provides a cleaner API than manually creating CustomEvent instances
 *
 * @param {string} eventName - Event name (should use Events constants from events.js)
 * @param {*} detail - Event detail payload (optional)
 *
 * @example
 * import { Events } from '../components/events.js';
 * import { dispatchEvent } from '../utils/api.js';
 *
 * dispatchEvent(Events.BOARD_DATA_LOADED, {
 *     board: boardData,
 *     lists: listsData
 * });
 */
function dispatchEvent(eventName, detail = null) {
    const event = detail !== null
        ? new CustomEvent(eventName, { detail })
        : new CustomEvent(eventName);
    document.dispatchEvent(event);
}

// Export all functions
export {
    apiCall,
    login,
    logout,
    getCurrentUser,
    getLists,
    createList,
    updateList,
    deleteList,
    reorderLists,
    copyOrMoveList,
    getBookmarks,
    createBookmark,
    updateBookmark,
    deleteBookmark,
    reorderBookmarks,
    getItems,
    createItem,
    updateItem,
    deleteItem,
    reorderItems,
    getBoards,
    getBoard,
    getBoardData,
    createBoard,
    updateBoard,
    deleteBoard,
    getAllData,
    exportData,
    importData,
    debounce,
    escapeHtml,
    showError,
    hideError,
    dispatchEvent
};
