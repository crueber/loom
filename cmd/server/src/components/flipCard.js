// Shared Card Flip Behavior
// Uses Alpine.store() for shared state management

import { Events } from './events.js';
import { dispatchEvent } from '../utils/api.js';

/**
 * Initialize the flip card Alpine store
 * Must be called during alpine:init
 */
export function initFlipCardStore() {
    Alpine.store('flipCard', {
        // Currently flipped card (null or { type: 'list'|'bookmark'|'note', id: string|number })
        currentlyFlipped: null,

        // Sortable instances for drag-and-drop
        listsSortable: null,
        bookmarkSortables: {},

        // Getters
        get hasFlippedCard() {
            return this.currentlyFlipped !== null;
        },

        get flippedType() {
            return this.currentlyFlipped?.type || null;
        },

        get flippedId() {
            return this.currentlyFlipped?.id || null;
        },

        // Methods
        setCurrentlyFlipped(card) {
            this.currentlyFlipped = card;
        },

        clearCurrentlyFlipped() {
            this.currentlyFlipped = null;
        },

        setListsSortable(sortable) {
            this.listsSortable = sortable;
        },

        setBookmarkSortable(listId, sortable) {
            this.bookmarkSortables[listId] = sortable;
        },

        disableAllSortables() {
            if (this.listsSortable) {
                this.listsSortable.option("disabled", true);
            }
            Object.values(this.bookmarkSortables).forEach(sortable => {
                sortable.option("disabled", true);
            });
        },

        enableAllSortables() {
            if (this.listsSortable) {
                this.listsSortable.option("disabled", false);
            }
            Object.values(this.bookmarkSortables).forEach(sortable => {
                sortable.option("disabled", false);
            });
        }
    });
}

/**
 * Get the flip card store
 * @returns {Object} Alpine store
 */
function _getStore() {
    return Alpine.store('flipCard');
}

/**
 * Close the currently flipped card
 */
export function closeFlippedCard() {
    const store = _getStore();
    const currentlyFlipped = store.currentlyFlipped;

    if (!currentlyFlipped) return;

    if (currentlyFlipped.type === 'list') {
        const card = document.querySelector(`.list-card[data-list-id="${currentlyFlipped.id}"]`);
        if (card) {
            // Remove if temp
            if (card.dataset.isTemp === 'true') {
                // Remove from lists array - will be handled by component
                dispatchEvent(Events.REMOVE_TEMP_LIST, { id: currentlyFlipped.id });
            } else {
                card.dataset.flipped = 'false';
            }
        }
    } else if (currentlyFlipped.type === 'bookmark') {
        const bookmark = document.querySelector(`.bookmark-item[data-item-id="${currentlyFlipped.id}"]`);
        if (bookmark) {
            // Remove if temp
            if (bookmark.dataset.isTemp === 'true') {
                bookmark.remove();
            } else {
                bookmark.dataset.flipped = 'false';
            }
        }
    } else if (currentlyFlipped.type === 'note') {
        const note = document.querySelector(`.note-item[data-item-id="${currentlyFlipped.id}"]`);
        if (note) {
            // Remove if temp
            if (note.dataset.isTemp === 'true') {
                note.remove();
            } else {
                note.dataset.flipped = 'false';
            }
        }
    }

    store.clearCurrentlyFlipped();
    store.enableAllSortables();
}

/**
 * Flip a list card to show configuration panel
 * @param {string|number} listId - List ID
 */
export function flipToList(listId) {
    const store = _getStore();

    // Close currently flipped card if it's different
    if (store.currentlyFlipped && (store.currentlyFlipped.type !== 'list' || store.currentlyFlipped.id !== listId)) {
        closeFlippedCard();
    }

    const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
    if (!card) return;

    card.dataset.flipped = 'true';
    store.setCurrentlyFlipped({ type: 'list', id: listId });

    // Disable all Sortable instances
    store.disableAllSortables();

    // Focus on title input and move cursor to end
    setTimeout(() => {
        const titleInput = card.querySelector('.config-list-title');
        if (titleInput) {
            titleInput.focus();
            const length = titleInput.value.length;
            titleInput.setSelectionRange(length, length);
        }
    }, 0);

    // Dispatch event for component to handle form reset
    dispatchEvent(Events.LIST_FLIPPED, { listId });
}

/**
 * Flip a bookmark card to show configuration panel
 * @param {string|number} bookmarkId - Bookmark ID
 */
export function flipToBookmark(bookmarkId) {
    const store = _getStore();

    // Close currently flipped card if it's different
    if (store.currentlyFlipped && (store.currentlyFlipped.type !== 'bookmark' || store.currentlyFlipped.id !== bookmarkId)) {
        closeFlippedCard();
    }

    const bookmarkEl = document.querySelector(`.bookmark-item[data-item-id="${bookmarkId}"]`);
    if (!bookmarkEl) return;

    bookmarkEl.dataset.flipped = 'true';
    store.setCurrentlyFlipped({ type: 'bookmark', id: bookmarkId });

    // Disable all Sortable instances
    store.disableAllSortables();

    // Dispatch event for component to handle form reset
    dispatchEvent(Events.BOOKMARK_FLIPPED, { bookmarkId });
}

/**
 * Flip a note card to show edit view
 * @param {string|number} noteId - Note ID
 */
export function flipToNote(noteId) {
    const store = _getStore();

    // Close currently flipped card if it's different
    if (store.currentlyFlipped && (store.currentlyFlipped.type !== 'note' || store.currentlyFlipped.id !== noteId)) {
        closeFlippedCard();
    }

    const noteEl = document.querySelector(`.note-item[data-item-id="${noteId}"]`);
    if (!noteEl) return;

    noteEl.dataset.flipped = 'true';
    store.setCurrentlyFlipped({ type: 'note', id: noteId });

    const textarea = noteEl.querySelector(`textarea`);
    textarea.focus();
    textarea.selectionStart = textarea.value.length;

    // Disable all Sortable instances
    store.disableAllSortables();

    // Dispatch event for component to handle form reset
    dispatchEvent(Events.NOTE_FLIPPED, { noteId });
}

/**
 * Initialize keyboard listener (runs once when module is loaded)
 * Call this from app.js after Alpine is initialized
 */
export function initFlipCardListeners() {
    document.addEventListener('keydown', (e) => {
        const store = _getStore();
        if (e.key === 'Escape' && store.hasFlippedCard) {
            closeFlippedCard();
        }
    });
}

// All components now use Alpine.store('flipCard') directly
// Legacy exports removed - use Alpine.store('flipCard') methods instead
