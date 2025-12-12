// Data Bootstrap Component
// Handles initial data loading for the application
// Determines which board to load based on URL and triggers data load

import { loadFromCache, saveToCache, hasDataChanged } from './cache.js';
import { dispatchEvent } from '../utils/api.js';
import { Events } from './events.js';

/**
 * Bootstrap application data on page load
 * - Parses browser URL to determine target board
 * - Loads from cache for instant display
 * - Fetches fresh data from server
 * - Dispatches events for other components to consume
 */
export async function bootstrapData() {
    const path = window.location.pathname;
    const match = path.match(/^\/boards\/(\d+)$/);
    const boardId = match ? parseInt(match[1]) : null;
    await loadBoardData(boardId);
}

/**
 * Load data for a specific board
 * @param {number} boardId - The board ID to load
 */
async function loadBoardData(boardId) {
    try {
        // Dispatch immediately for instant load when cached.
        const cachedData = loadFromCache();
        if (cachedData && cachedData.board && (cachedData.board.id === boardId || cachedData.board.is_default)) {
            dispatchBoardDataLoaded({
                board: cachedData.board,
                boards: cachedData.boards,
                lists: cachedData.lists,
                items: cachedData.items || cachedData.bookmarks
            });
        }

        // Fetch fresh data in background.
        if (!boardId) {
            boardId = await getDefaultBoardId();
        }
        const response = await fetch(`/api/boards/${boardId}/data`);
        if (response.status === 404) {
            window.location.href = '/';
            return;
        }
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const freshData = await response.json();

        // Dispatch if data has changed.
        if (!cachedData || hasDataChanged(cachedData, freshData)) {
            // Dispatch fresh data
            dispatchBoardDataLoaded({
                board: freshData.board,
                boards: freshData.boards,
                lists: freshData.lists,
                items: freshData.items
            });

            // Save to cache for next load
            saveToCache(freshData);
        }
    } catch (error) {
        console.error('Failed to load board data:', error);
        const cachedData = loadFromCache();
        if (!cachedData) {
            alert('Failed to load board. Redirecting to default.');
            window.location.href = '/';
        }
    }
}

/**
 * Load the default board
 * Tries cache first, falls back to API call to find default board
 */
async function getDefaultBoardId() {
    const cachedData = loadFromCache();
    if (cachedData && cachedData.board && cachedData.board.is_default) {
        return cachedData.board.id;
    }

    const response = await fetch('/api/boards');
    if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
    }
    const boards = await response.json();
    const defaultBoard = boards.find(b => b.is_default);
    return defaultBoard.id
}

/**
 * Dispatch board data loaded event
 * @param {Object} data - Board data to dispatch
 */
function dispatchBoardDataLoaded(data) {
    dispatchEvent(Events.BOARD_DATA_LOADED, data);
}
