// Data Bootstrap Component
// Handles initial data loading for the application
// Determines which board to load based on URL and triggers data load

import { dispatchEvent } from '../utils/api.js';
import { Events } from './events.js';

/**
 * Bootstrap application data on page load
 */
export async function bootstrapData() {
    try {
        // Check for server-side bootstrapped data and if the user is logged in
        if (!window.__BOOTSTRAP_DATA__ || !window.__BOOTSTRAP_DATA__.user) {
            return dispatchEvent(Events.USER_LOGGED_OUT);
        }
        const bootstrapData = window.__BOOTSTRAP_DATA__;

        // Dispatch user logged in event
        dispatchEvent(Events.USER_LOGGED_IN, { user: bootstrapData.user });

        // Dispatch board data
        dispatchEvent(Events.BOARD_DATA_LOADED, {
            board: bootstrapData.board,
            boards: bootstrapData.boards,
            lists: bootstrapData.lists,
            items: bootstrapData.items
        });

        // Clear bootstrap data so we don't use it again
        delete window.__BOOTSTRAP_DATA__;
    } catch (error) {
        console.error('Unable to bootstrap:', error);
        // Show login screen on error
    }
}
