// Data Bootstrap Component
// Handles initial data loading for the application
// Determines which board to load based on URL and triggers data load

import { dispatchEvent } from '../utils/api.js';
import { Events } from './events.js';

/**
 * Bootstrap application data on page load
 * - Parses browser URL to determine target board
 * - Uses server-side bootstrapped data for instant display
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
        // Check for server-side bootstrapped data
        if (window.__BOOTSTRAP_DATA__) {
            const bootstrapData = window.__BOOTSTRAP_DATA__;

            // Use bootstrapped data immediately for instant load
            dispatchBoardDataLoaded({
                board: bootstrapData.board,
                boards: bootstrapData.boards,
                lists: bootstrapData.lists,
                items: bootstrapData.items
            });

            // Clear bootstrap data so we don't use it again
            delete window.__BOOTSTRAP_DATA__;
        } else {
            // No bootstrap data - fetch from API
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

            // Dispatch data
            dispatchBoardDataLoaded({
                board: freshData.board,
                boards: freshData.boards,
                lists: freshData.lists,
                items: freshData.items
            });
        }
    } catch (error) {
        console.error('Failed to load board data:', error);
        if (!window.__BOOTSTRAP_DATA__) {
            alert('Failed to load board. Redirecting to default.');
            window.location.href = '/';
        }
    }
}

/**
 * Load the default board ID from API
 */
async function getDefaultBoardId() {
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
