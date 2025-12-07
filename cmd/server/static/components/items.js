// Items (Bookmarks) Management Component
// Note: Currently handles bookmarks, prepared for Phase 3 conversion to unified Items (links & notes)

document.addEventListener('alpine:init', () => {
    Alpine.data('itemsManager', () => ({
    bookmarks: {},
    bookmarkSortables: {},

    init() {
        // Listen for bookmarks data loaded from lists manager
        document.addEventListener('bookmarksDataLoaded', (event) => {
            this.bookmarks = event.detail.bookmarks;
        });

        // Listen for render request for specific list
        document.addEventListener('renderListBookmarks', (event) => {
            this.renderBookmarks(event.detail.listId);
        });

        // Listen for add bookmark request
        document.addEventListener('addBookmarkRequested', (event) => {
            this.addBookmark(event.detail.listId);
        });

        // Listen for list deletion to clean up bookmarks
        document.addEventListener('listDeleted', (event) => {
            delete this.bookmarks[event.detail.listId];
            this.updateCache();
        });

        // Listen for lists updates to maintain cache
        document.addEventListener('listsUpdated', (event) => {
            const cachedData = loadFromCache();
            if (cachedData) {
                saveToCache({ lists: event.detail.lists, bookmarks: this.bookmarks });
            }
        });

        // Listen for bookmark flipped event
        document.addEventListener('bookmarkFlipped', () => {
            this.disableAllSortables();
        });

        // Listen for user logout
        document.addEventListener('userLoggedOut', () => {
            this.bookmarks = {};
            Object.values(this.bookmarkSortables).forEach(sortable => sortable.destroy());
            this.bookmarkSortables = {};
        });
    },

    renderBookmarks(listId) {
        const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
        if (!container) return;

        const listBookmarks = this.bookmarks[listId] || [];
        container.innerHTML = '';

        listBookmarks.forEach(bookmark => {
            const bookmarkEl = this.createBookmarkElement(bookmark);
            container.appendChild(bookmarkEl);
        });

        // Initialize Sortable for bookmarks
        this.initializeSortable(listId, container);
    },

    initializeSortable(listId, container) {
        // Destroy old instance if exists
        if (this.bookmarkSortables[listId]) {
            this.bookmarkSortables[listId].destroy();
        }

        this.bookmarkSortables[listId] = new Sortable(container, {
            group: 'bookmarks',
            animation: 150,
            handle: '.bookmark-card-front',
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
            onEnd: (evt) => this.handleBookmarkReorder(evt),
            onMove: (evt) => {
                // Don't allow dragging if the item is flipped
                return evt.related.dataset.flipped !== 'true';
            }
        });

        // Store reference globally for flipCard component
        bookmarkSortables[listId] = this.bookmarkSortables[listId];
    },

    disableAllSortables() {
        Object.values(this.bookmarkSortables).forEach(sortable => {
            sortable.option("disabled", true);
        });
    },

    enableAllSortables() {
        Object.values(this.bookmarkSortables).forEach(sortable => {
            sortable.option("disabled", false);
        });
    },

    createBookmarkElement(bookmark) {
        const wrapper = document.createElement('div');
        wrapper.className = 'bookmark-item';
        wrapper.dataset.bookmarkId = bookmark.id;
        wrapper.dataset.listId = bookmark.list_id;
        wrapper.dataset.flipped = 'false';

        // Mark temp bookmarks
        if (typeof bookmark.id === 'string' && bookmark.id.startsWith('temp-')) {
            wrapper.dataset.isTemp = 'true';
        }

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
            this.deleteBookmarkFromConfig(bookmark.id);
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

    async saveBookmarkConfig(bookmarkId) {
        const bookmarkEl = document.querySelector(`.bookmark-item[data-bookmark-id="${bookmarkId}"]`);
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
                const bookmark = await createBookmark(listId, newTitle, newUrl);
                if (!this.bookmarks[listId]) this.bookmarks[listId] = [];
                this.bookmarks[listId].push(bookmark);
                this.updateCache();
                this.renderBookmarks(listId);
                closeFlippedCard();
            } catch (error) {
                console.error('Failed to create bookmark:', error);
                alert('Failed to create bookmark: ' + error.message);
            }
            return;
        }

        // Find bookmark
        let bookmark = null;
        for (const listId in this.bookmarks) {
            bookmark = this.bookmarks[listId].find(b => b.id === bookmarkId);
            if (bookmark) break;
        }
        if (!bookmark) return;

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

                // Update cache
                this.updateCache();

                // Re-render to update front
                this.renderBookmarks(bookmark.list_id);
            } else {
                closeFlippedCard();
            }
        } catch (error) {
            console.error('Failed to save bookmark:', error);
            alert('Failed to save changes');
        }
    },

    async deleteBookmarkFromConfig(bookmarkId) {
        const bookmarkEl = document.querySelector(`.bookmark-item[data-bookmark-id="${bookmarkId}"]`);
        if (!bookmarkEl) return;

        // Find bookmark
        let bookmark = null;
        for (const listId in this.bookmarks) {
            bookmark = this.bookmarks[listId].find(b => b.id === bookmarkId);
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
                for (const listId in this.bookmarks) {
                    this.bookmarks[listId] = this.bookmarks[listId].filter(b => b.id !== bookmarkId);
                }

                // Update cache
                this.updateCache();

                // Clear flipped card state
                setCurrentlyFlippedCard(null);

                // Remove element
                bookmarkEl.remove();
            } catch (error) {
                console.error('Failed to delete bookmark:', error);
                alert('Failed to delete bookmark');
            }
        });
    },

    async addBookmark(listId) {
        const tempId = `temp-${Date.now()}`;
        const tempBookmark = {
            id: tempId,
            list_id: listId,
            title: '',
            url: '',
            favicon_url: null
        };

        const wrapper = this.createBookmarkElement(tempBookmark);
        wrapper.dataset.isTemp = 'true';
        wrapper.dataset.flipped = 'true';

        const container = document.querySelector(`.bookmarks-container[data-list-id="${listId}"]`);
        container.appendChild(wrapper);

        flipToBookmark(tempId);

        // Focus first input
        setTimeout(() => wrapper.querySelector('.config-bookmark-title').focus(), 300);
    },

    async handleBookmarkReorder(evt) {
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
            for (const listId in this.bookmarks) {
                const bookmark = this.bookmarks[listId].find(b => b.id === bookmarkId);
                if (bookmark) {
                    this.bookmarks[listId] = this.bookmarks[listId].filter(b => b.id !== bookmarkId);
                    bookmark.list_id = newListId;
                    bookmark.position = reorderedBookmarks.find(r => r.id === bookmarkId).position;

                    if (!this.bookmarks[newListId]) this.bookmarks[newListId] = [];
                    this.bookmarks[newListId].push(bookmark);
                    break;
                }
            }

            // Update all affected lists
            const affectedListIds = new Set([...reorderedBookmarks.map(r => r.list_id)]);
            affectedListIds.forEach(listId => {
                this.bookmarks[listId].sort((a, b) => a.position - b.position);
            });

            // Update cache
            this.updateCache();
        } catch (error) {
            console.error('Failed to reorder bookmarks:', error);
            // Reload on failure - dispatch event to lists manager
            const event = new CustomEvent('reloadDataRequested');
            document.dispatchEvent(event);
        }
    },

    updateCache() {
        // Get current lists from cache
        const cachedData = loadFromCache();
        if (cachedData) {
            saveToCache({ lists: cachedData.lists, bookmarks: this.bookmarks });
        }
    }
}));
});
