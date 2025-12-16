/**
 * Event Registry
 *
 * Centralized registry of all CustomEvent names used in the application.
 * Use these constants instead of string literals to prevent typos and
 * enable IDE autocomplete.
 *
 * Naming convention: SCREAMING_SNAKE_CASE, past tense (e.g., DATA_LOADED, USER_LOGGED_IN)
 */

export const Events = {
    // ============================================================
    // Authentication Events
    // ============================================================

    /**
     * Dispatched when user successfully logs in via OAuth2
     * @event USER_LOGGED_IN
     * @property {Object} detail.user - The logged-in user object
     */
    USER_LOGGED_IN: 'userLoggedIn',

    /**
     * Dispatched when user logs out
     * @event USER_LOGGED_OUT
     */
    USER_LOGGED_OUT: 'userLoggedOut',

    // ============================================================
    // Data Loading Events
    // ============================================================

    /**
     * Dispatched when board data is loaded from server
     * Includes the current board, all boards list, lists, and items
     * @event BOARD_DATA_LOADED
     * @property {Object} detail.board - Current board object
     * @property {Array} detail.boards - All boards for the user
     * @property {Array} detail.lists - Lists for the current board
     * @property {Array} detail.items - Items for all lists
     */
    BOARD_DATA_LOADED: 'boardDataLoaded',

    /**
     * Dispatched when boards list is loaded
     * Legacy event for backwards compatibility
     * @event BOARDS_DATA_LOADED
     * @property {Array} detail.boards - All boards for the user
     * @deprecated Use BOARD_DATA_LOADED instead
     */
    BOARDS_DATA_LOADED: 'boardsDataLoaded',

    /**
     * Dispatched when bookmarks/items data is loaded
     * Legacy event for backwards compatibility with items component
     * @event BOOKMARKS_DATA_LOADED
     * @property {Object} detail.items - Items keyed by list_id
     */
    BOOKMARKS_DATA_LOADED: 'bookmarksDataLoaded',

    // ============================================================
    // List Events
    // ============================================================

    /**
     * Dispatched when lists are updated (created, reordered, etc.)
     * Can be used by components that need to react to list changes
     * @event LISTS_UPDATED
     * @property {Array} detail.lists - Updated lists array
     */
    LISTS_UPDATED: 'listsUpdated',

    /**
     * Dispatched when a list is deleted
     * Notifies items component to clean up
     * @event LIST_DELETED
     * @property {string|number} detail.listId - ID of deleted list
     */
    LIST_DELETED: 'listDeleted',

    /**
     * Dispatched when a list card is flipped to configuration view
     * @event LIST_FLIPPED
     * @property {string|number} detail.listId - ID of flipped list
     */
    LIST_FLIPPED: 'listFlipped',

    /**
     * Dispatched to request removal of a temporary list
     * @event REMOVE_TEMP_LIST
     * @property {string} detail.id - Temporary list ID (temp-{timestamp})
     */
    REMOVE_TEMP_LIST: 'removeTempList',

    // ============================================================
    // Item (Bookmark/Note) Events
    // ============================================================

    /**
     * Dispatched to request rendering of bookmarks for a specific list
     * @event RENDER_LIST_BOOKMARKS
     * @property {string|number} detail.listId - List ID to render items for
     */
    RENDER_LIST_BOOKMARKS: 'renderListBookmarks',

    /**
     * Dispatched to request adding a new bookmark to a list
     * @event ADD_BOOKMARK_REQUESTED
     * @property {string|number} detail.listId - List ID to add bookmark to
     */
    ADD_BOOKMARK_REQUESTED: 'addBookmarkRequested',

    /**
     * Dispatched to request adding a new note to a list
     * @event ADD_NOTE_REQUESTED
     * @property {string|number} detail.listId - List ID to add note to
     */
    ADD_NOTE_REQUESTED: 'addNoteRequested',

    /**
     * Dispatched when a bookmark card is flipped to configuration view
     * @event BOOKMARK_FLIPPED
     * @property {string|number} detail.bookmarkId - ID of flipped bookmark
     */
    BOOKMARK_FLIPPED: 'bookmarkFlipped',

    /**
     * Dispatched when a note card is flipped to edit view
     * @event NOTE_FLIPPED
     * @property {string|number} detail.noteId - ID of flipped note
     */
    NOTE_FLIPPED: 'noteFlipped',

    /**
     * Dispatched when bookmarks are updated
     * Legacy event for cache updates
     * @event BOOKMARKS_UPDATED
     * @property {Array} detail.bookmarks - Updated bookmarks array
     * @deprecated Use LISTS_UPDATED instead
     */
    BOOKMARKS_UPDATED: 'bookmarksUpdated',
};

/**
 * Get all event names as an array
 * Useful for debugging or validation
 */
export function getAllEventNames() {
    return Object.values(Events);
}

/**
 * Check if a string is a valid event name
 * @param {string} eventName - Event name to validate
 * @returns {boolean}
 */
export function isValidEvent(eventName) {
    return getAllEventNames().includes(eventName);
}
