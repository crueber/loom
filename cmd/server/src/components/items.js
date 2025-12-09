// Items Management Component (Bookmarks & Notes)

import { createItem, updateItem, deleteItem, reorderItems, escapeHtml } from '../utils/api.js';
import { flipToBookmark, flipToNote, closeFlippedCard, setBookmarkSortable } from './flipCard.js';
import { loadFromCache, saveToCache } from './cache.js';

// Helper function to process color tags in note content
// Converts [color=X]...[/color] to <span style="color: X">...</span>
// The inner content can still contain markdown which will be processed by marked.js
function processColorTags(content) {
    // Match [color=VALUE]...[/color] where VALUE can be:
    // - HTML color names (red, blue, yellow, etc.)
    // - Hex colors (#123456 or #123)
    const colorRegex = /\[color=([^\]]+)\]([\s\S]*?)\[\/color\]/gi;

    return content.replace(colorRegex, (_match, color, text) => {
        // Validate color format (hex or named color)
        const isValidHex = /^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$/.test(color);
        const isValidNamed = /^[a-z]+$/i.test(color);

        if (isValidHex || isValidNamed) {
            // Convert to HTML span - marked.js will preserve this if we enable HTML
            return `<span style="color: ${color}">${text}</span>`;
        }

        // If invalid color, return just the text without styling
        return text;
    });
}

document.addEventListener('alpine:init', () => {
    Alpine.data('itemsManager', () => ({
    items: {},  // Changed from bookmarks to items
    itemSortables: {},  // Changed from bookmarkSortables

    init() {
        // Listen for items data loaded from lists manager (backward compat with bookmarks)
        document.addEventListener('bookmarksDataLoaded', (event) => {
            // Convert bookmarks to items format if needed
            const bookmarks = event.detail.bookmarks;
            for (const listId in bookmarks) {
                if (!this.items[listId]) this.items[listId] = [];
                this.items[listId] = bookmarks[listId].map(b => ({
                    ...b,
                    type: b.type || 'bookmark'  // Add type if missing
                }));
            }
        });

        // Listen for render request for specific list
        document.addEventListener('renderListBookmarks', (event) => {
            this.renderItems(event.detail.listId);
        });

        // Listen for add bookmark request
        document.addEventListener('addBookmarkRequested', (event) => {
            this.addItem(event.detail.listId, 'bookmark');
        });

        // Listen for add note request
        document.addEventListener('addNoteRequested', (event) => {
            this.addItem(event.detail.listId, 'note');
        });

        // Listen for list deletion to clean up items
        document.addEventListener('listDeleted', (event) => {
            delete this.items[event.detail.listId];
            this.updateCache();
        });

        // Listen for lists updates to maintain cache
        document.addEventListener('listsUpdated', (event) => {
            const cachedData = loadFromCache();
            if (cachedData) {
                saveToCache({ lists: event.detail.lists, items: this.items });
            }
        });

        // Listen for item flipped event
        document.addEventListener('bookmarkFlipped', () => {
            this.disableAllSortables();
        });

        document.addEventListener('noteFlipped', () => {
            this.disableAllSortables();
        });

        // Listen for user logout
        document.addEventListener('userLoggedOut', () => {
            this.items = {};
            Object.values(this.itemSortables).forEach(sortable => sortable.destroy());
            this.itemSortables = {};
        });
    },

    renderItems(listId) {
        const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
        if (!container) return;

        const listItems = this.items[listId] || [];
        container.innerHTML = '';

        listItems.forEach(item => {
            const itemEl = item.type === 'note'
                ? this.createNoteElement(item)
                : this.createBookmarkElement(item);
            container.appendChild(itemEl);
        });

        // Initialize Sortable for items
        this.initializeSortable(listId, container);
    },

    initializeSortable(listId, container) {
        // Destroy old instance if exists
        if (this.itemSortables[listId]) {
            this.itemSortables[listId].destroy();
        }

        this.itemSortables[listId] = new Sortable(container, {
            group: 'items',  // Changed from 'bookmarks'
            animation: 150,
            handle: '.bookmark-card-front, .note-card-front',
            filter: '[data-flipped="true"]',
            delay: 200,
            delayOnTouchOnly: true,
            dragClass: 'sortable-drag',
            onStart: (evt) => {
                // If any card is flipped, cancel the drag
                if (getCurrentlyFlippedCard()) {
                    evt.item.dispatchEvent(new Event('mouseup'));
                    return false;
                }
            },
            onEnd: (evt) => this.handleItemReorder(evt),
            onMove: (evt) => {
                // Don't allow dragging if the item is flipped
                return evt.related.dataset.flipped !== 'true';
            }
        });

        // Store reference globally for flipCard component
        setBookmarkSortable(listId, this.itemSortables[listId]);
    },

    disableAllSortables() {
        Object.values(this.itemSortables).forEach(sortable => {
            sortable.option("disabled", true);
        });
    },

    enableAllSortables() {
        Object.values(this.itemSortables).forEach(sortable => {
            sortable.option("disabled", false);
        });
    },

    createBookmarkElement(bookmark) {
        const wrapper = document.createElement('div');
        wrapper.className = 'bookmark-item';
        wrapper.dataset.itemId = bookmark.id;
        wrapper.dataset.bookmarkId = bookmark.id;  // Keep for backward compat
        wrapper.dataset.listId = bookmark.list_id;
        wrapper.dataset.flipped = 'false';
        wrapper.dataset.type = 'bookmark';

        // Mark temp bookmarks
        if (typeof bookmark.id === 'string' && bookmark.id.startsWith('temp-')) {
            wrapper.dataset.isTemp = 'true';
        }

        const faviconHtml = bookmark.favicon_url
            ? `<img src="${escapeHtml(bookmark.favicon_url)}" alt="">`
            : '<span class="bookmark-favicon-placeholder">📄</span>';

        wrapper.innerHTML = `
            <div class="bookmark-card-inner">
                <a href="${escapeHtml(bookmark.url)}" rel="noopener noreferrer" class="bookmark-card-front">
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
                        <label for="config-bookmark-title-${bookmark.id}">
                            Title
                            <input type="text" id="config-bookmark-title-${bookmark.id}" name="bookmark-title" class="config-bookmark-title" placeholder="Title" value="${escapeHtml(bookmark.title)}" autocomplete="off">
                        </label>
                        <label for="config-bookmark-url-${bookmark.id}">
                            URL
                            <input type="url" id="config-bookmark-url-${bookmark.id}" name="bookmark-url" class="config-bookmark-url" placeholder="URL" value="${escapeHtml(bookmark.url)}" autocomplete="off">
                        </label>
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
            this.saveBookmarkConfig(bookmark.id);
        });

        // Delete button
        wrapper.querySelector('.bookmark-config-delete-btn').addEventListener('click', (e) => {
            e.preventDefault();
            this.deleteItemFromConfig(bookmark.id, 'bookmark');
        });

        // Enter key to save on either input
        const titleInput = wrapper.querySelector('.config-bookmark-title');
        const urlInput = wrapper.querySelector('.config-bookmark-url');

        titleInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.saveBookmarkConfig(bookmark.id);
            }
        });

        urlInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.saveBookmarkConfig(bookmark.id);
            }
        });

        return wrapper;
    },

    createNoteElement(note) {
        const wrapper = document.createElement('div');
        wrapper.className = 'note-item';
        wrapper.dataset.itemId = note.id;
        wrapper.dataset.listId = note.list_id;
        wrapper.dataset.flipped = 'false';
        wrapper.dataset.type = 'note';

        // Mark temp notes
        if (typeof note.id === 'string' && note.id.startsWith('temp-')) {
            wrapper.dataset.isTemp = 'true';
        }

        const noteContent = note.content || '';
        // Process color tags first (converts [color=X]...[/color] to HTML spans)
        const processedContent = noteContent ? processColorTags(noteContent) : '';
        // Then render markdown with HTML enabled so our color spans are preserved
        const renderedContent = processedContent ? marked.parse(processedContent, {
            breaks: true,        // Convert \n to <br> for better formatting
            gfm: false,         // Disable GitHub Flavored Markdown extras
            headerIds: false,   // Disable auto-generated header IDs
            mangle: false,      // Don't encode email addresses
        }) : '';

        wrapper.innerHTML = `
            <div class="note-card-inner">
                <div class="note-card-front">
                    <div class="note-content">
                        <div class="note-text">${renderedContent}</div>
                    </div>
                </div>
                <div class="note-card-back">
                    <div class="note-config-panel">
                        <textarea id="config-note-content-${note.id}" class="config-note-content" placeholder="Enter note text...">${escapeHtml(noteContent)}</textarea>
                        <div class="config-actions">
                            <button class="config-delete-btn secondary">Delete</button>
                            <button class="config-cancel-btn secondary">Cancel</button>
                            <button class="config-save-btn">Save</button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        // Gear icon - flip to back
        wrapper.querySelector('.note-card-front').addEventListener('click', (e) => {
            e.preventDefault();
            e.stopPropagation();
            flipToNote(note.id);
        });

        // Cancel button
        wrapper.querySelector('.config-cancel-btn').addEventListener('click', (e) => {
            e.preventDefault();
            closeFlippedCard();
        });

        // Save button
        wrapper.querySelector('.config-save-btn').addEventListener('click', (e) => {
            e.preventDefault();
            this.saveNoteConfig(note.id);
        });

        // Delete button
        wrapper.querySelector('.config-delete-btn').addEventListener('click', (e) => {
            e.preventDefault();
            this.deleteItemFromConfig(note.id, 'note');
        });

        // Keyboard handling for textarea
        const textarea = wrapper.querySelector('.config-note-content');
        textarea.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                // Enter saves, Shift+Enter adds new line
                e.preventDefault();
                this.saveNoteConfig(note.id);
            }
        });

        return wrapper;
    },

    async saveBookmarkConfig(bookmarkId) {
        const bookmarkEl = document.querySelector(`.bookmark-item[data-item-id="${bookmarkId}"]`);
        if (!bookmarkEl) return;

        const isTemp = bookmarkEl.dataset.isTemp === 'true';
        const newTitle = bookmarkEl.querySelector('.config-bookmark-title').value.trim();
        const newUrl = bookmarkEl.querySelector('.config-bookmark-url').value.trim();

        if (isTemp) {
            // Cancel if empty
            if (!newTitle || !newUrl) {
                bookmarkEl.remove();
                closeFlippedCard();
                return;
            }

            // Create new bookmark
            const listId = parseInt(bookmarkEl.dataset.listId);
            try {
                const item = await createItem(listId, 'bookmark', { title: newTitle, url: newUrl });
                if (!this.items[listId]) this.items[listId] = [];
                this.items[listId].push(item);
                this.updateCache();
                this.renderItems(listId);
                closeFlippedCard();
            } catch (error) {
                console.error('Failed to create bookmark:', error);
                alert('Failed to create bookmark: ' + error.message);
            }
            return;
        }

        // Find item
        let item = null;
        for (const listId in this.items) {
            item = this.items[listId].find(i => i.id == bookmarkId && i.type === 'bookmark');
            if (item) break;
        }

        if (!item) return;

        // Check if anything changed
        const updates = {};
        if (newTitle !== item.title) updates.title = newTitle;
        if (newUrl !== item.url) updates.url = newUrl;

        if (Object.keys(updates).length === 0) {
            closeFlippedCard();
            return;
        }

        try {
            const updatedItem = await updateItem(item.id, updates);
            Object.assign(item, updatedItem);
            this.updateCache();
            this.renderItems(item.list_id);
            closeFlippedCard();
        } catch (error) {
            console.error('Failed to update bookmark:', error);
            alert('Failed to update bookmark: ' + error.message);
        }
    },

    async saveNoteConfig(noteId) {
        const noteEl = document.querySelector(`.note-item[data-item-id="${noteId}"]`);
        if (!noteEl) return;

        const isTemp = noteEl.dataset.isTemp === 'true';
        const newContent = noteEl.querySelector('.config-note-content').value.trim();

        if (isTemp) {
            // Cancel if empty
            if (!newContent) {
                noteEl.remove();
                closeFlippedCard();
                return;
            }

            // Create new note
            const listId = parseInt(noteEl.dataset.listId);
            try {
                const item = await createItem(listId, 'note', { content: newContent });
                if (!this.items[listId]) this.items[listId] = [];
                this.items[listId].push(item);
                this.updateCache();
                this.renderItems(listId);
                closeFlippedCard();
            } catch (error) {
                console.error('Failed to create note:', error);
                alert('Failed to create note: ' + error.message);
            }
            return;
        }

        // Find item
        let item = null;
        for (const listId in this.items) {
            item = this.items[listId].find(i => i.id == noteId && i.type === 'note');
            if (item) break;
        }

        if (!item) return;

        // Delete if content is empty
        if (!newContent) {
            await this.deleteItemFromConfig(noteId, 'note', true);
            return;
        }

        // Check if anything changed
        if (newContent === item.content) {
            closeFlippedCard();
            return;
        }

        try {
            const updatedItem = await updateItem(item.id, { content: newContent });
            Object.assign(item, updatedItem);
            this.updateCache();
            this.renderItems(item.list_id);
            closeFlippedCard();
        } catch (error) {
            console.error('Failed to update note:', error);
            alert('Failed to update note: ' + error.message);
        }
    },

    async deleteItemFromConfig(itemId, itemType, skipConfirm = false) {
        const selector = itemType === 'note' ? '.note-item' : '.bookmark-item';
        const itemEl = document.querySelector(`${selector}[data-item-id="${itemId}"]`);
        if (!itemEl) return;

        const deleteBtn = itemEl.querySelector('.config-delete-btn, .bookmark-config-delete-btn');

        if (!skipConfirm && deleteBtn.textContent !== 'Confirm Delete?') {
            deleteBtn.textContent = 'Confirm Delete?';
            setTimeout(() => {
                if (deleteBtn) deleteBtn.textContent = 'Delete';
            }, 3000);
            return;
        }

        try {
            await deleteItem(itemId);

            // Remove from local state
            for (const listId in this.items) {
                const index = this.items[listId].findIndex(i => i.id == itemId);
                if (index !== -1) {
                    this.items[listId].splice(index, 1);
                    this.updateCache();
                    this.renderItems(parseInt(listId));
                    closeFlippedCard();
                    break;
                }
            }
        } catch (error) {
            console.error('Failed to delete item:', error);
            alert('Failed to delete item');
        }
    },

    async addItem(listId, type) {
        const tempId = `temp-${type}-${Date.now()}`;
        const tempItem = {
            id: tempId,
            list_id: listId,
            type: type
        };

        if (type === 'bookmark') {
            tempItem.title = '';
            tempItem.url = '';
            tempItem.favicon_url = null;
        } else if (type === 'note') {
            tempItem.content = '';
        }

        const wrapper = type === 'note'
            ? this.createNoteElement(tempItem)
            : this.createBookmarkElement(tempItem);

        wrapper.dataset.isTemp = 'true';
        wrapper.dataset.flipped = 'true';

        const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
        container.appendChild(wrapper);

        if (type === 'bookmark') {
            flipToBookmark(tempId);
            setTimeout(() => wrapper.querySelector('.config-bookmark-title').focus(), 300);
        } else {
            flipToNote(tempId);
            setTimeout(() => wrapper.querySelector('.config-note-content').focus(), 300);
        }
    },

    async handleItemReorder(evt) {
        const { from, to, item } = evt;
        const itemId = parseInt(item.dataset.itemId);
        const newListId = parseInt(to.dataset.listId);

        const itemElements = to.querySelectorAll('.bookmark-item, .note-item');
        const reorderedItems = Array.from(itemElements).map((el, index) => ({
            id: parseInt(el.dataset.itemId),
            position: index,
            list_id: newListId
        }));

        try {
            await reorderItems(reorderedItems);

            // Update local state
            for (const listId in this.items) {
                const itemObj = this.items[listId].find(i => i.id === itemId);
                if (itemObj) {
                    this.items[listId] = this.items[listId].filter(i => i.id !== itemId);
                    itemObj.list_id = newListId;
                    itemObj.position = reorderedItems.find(r => r.id === itemId).position;

                    if (!this.items[newListId]) this.items[newListId] = [];
                    this.items[newListId].push(itemObj);
                    break;
                }
            }

            // Update all affected lists
            const affectedListIds = new Set([...reorderedItems.map(r => r.list_id)]);
            affectedListIds.forEach(listId => {
                this.items[listId].sort((a, b) => a.position - b.position);
            });

            // Re-render all affected lists to reflect the new order
            affectedListIds.forEach(listId => {
                this.renderItems(listId);
            });

            // Update cache
            this.updateCache();
        } catch (error) {
            console.error('Failed to reorder items:', error);
            // Reload on failure - dispatch event to lists manager
            const event = new CustomEvent('reloadDataRequested');
            document.dispatchEvent(event);
        }
    },

    updateCache() {
        // Get current lists from cache
        const cachedData = loadFromCache();
        if (cachedData) {
            saveToCache({ lists: cachedData.lists, items: this.items });
        }
    }
}));
});
