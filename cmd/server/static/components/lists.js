// Lists Management Component

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

document.addEventListener('alpine:init', () => {
    Alpine.data('listsManager', () => ({
    lists: [],
    listsSortable: null,

    init() {
        // Listen for board data loaded (from boards manager)
        document.addEventListener('boardDataLoaded', (event) => {
            this.lists = event.detail.lists || [];

            // Dispatch bookmarks to items manager
            const bookmarksEvent = new CustomEvent('bookmarksDataLoaded', {
                detail: { bookmarks: event.detail.bookmarks }
            });
            document.dispatchEvent(bookmarksEvent);

            this.$nextTick(() => this.renderLists());
        });

        // Listen for user logout event
        document.addEventListener('userLoggedOut', () => {
            this.lists = [];
            if (this.listsSortable) {
                this.listsSortable.destroy();
                this.listsSortable = null;
            }
        });

        // Listen for flipCard events to update sortable state
        document.addEventListener('listFlipped', () => {
            this.disableSortable();
        });

        // Listen for removeTempList event
        document.addEventListener('removeTempList', (event) => {
            const index = this.lists.findIndex(l => l.id === event.detail.id);
            if (index !== -1) {
                this.lists.splice(index, 1);
                this.renderLists();
            }
        });
    },

    getBoardId() {
        // Get current board ID from boards manager
        const appEl = document.querySelector('[x-data*="boardsManager"]');
        if (appEl) {
            const boardsManager = Alpine.$data(appEl);
            if (boardsManager && boardsManager.currentBoard) {
                return boardsManager.currentBoard.id;
            }
        }
        return null;
    },

    async loadData() {
        // Step 1: Load from cache and render immediately
        const cachedData = loadFromCache();
        if (cachedData) {
            this.lists = cachedData.lists;

            // Dispatch cached bookmarks to items manager
            const cachedEvent = new CustomEvent('bookmarksDataLoaded', {
                detail: { bookmarks: cachedData.bookmarks }
            });
            document.dispatchEvent(cachedEvent);

            this.$nextTick(() => this.renderLists());
        }

        // Step 2: Fetch fresh data from server in background
        try {
            const response = await fetch('/api/data');
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            const freshData = await response.json();

            // Step 3: Check if data has changed or if no cache existed
            if (!cachedData || hasDataChanged(cachedData, freshData)) {
                this.lists = freshData.lists;

                // Dispatch event with bookmark data for items manager
                const event = new CustomEvent('bookmarksDataLoaded', {
                    detail: { bookmarks: freshData.bookmarks }
                });
                document.dispatchEvent(event);

                // Save to cache for next load
                saveToCache(freshData);

                // Re-render with fresh data
                this.$nextTick(() => this.renderLists());
            }
        } catch (error) {
            console.error('Failed to load data:', error);
            if (!cachedData) {
                alert('Failed to load bookmarks. Please refresh the page.');
            }
        }
    },

    renderLists() {
        const container = document.getElementById('lists-container');
        if (!container) return;

        // Remove only list cards, keep the add-list-container
        const listCards = container.querySelectorAll('.list-card');
        listCards.forEach(card => card.remove());

        // Insert lists before the add-list-container
        const addListContainer = document.getElementById('add-list-container');
        this.lists.forEach(list => {
            const listEl = this.createListElement(list);
            container.insertBefore(listEl, addListContainer);

            // Dispatch event to render bookmarks for this list
            const event = new CustomEvent('renderListBookmarks', { detail: { listId: list.id } });
            document.dispatchEvent(event);
        });

        // Initialize Sortable for lists
        this.initializeSortable();
    },

    initializeSortable() {
        const container = document.getElementById('lists-container');
        if (!container) return;

        // Destroy old instance if exists
        if (this.listsSortable) {
            this.listsSortable.destroy();
        }

        this.listsSortable = new Sortable(container, {
            animation: 150,
            handle: '.list-header',
            filter: '[data-flipped="true"]',
            scroll: true,
            scrollSensitivity: 100,
            scrollSpeed: 20,
            bubbleScroll: true,
            delay: 200,
            delayOnTouchOnly: true,
            onStart: (evt) => {
                // Hide the back of the card during drag by setting a custom ghost
                const listCard = evt.item;
                const cardBack = listCard.querySelector('.list-card-back');
                if (cardBack) {
                    cardBack.style.display = 'none';
                }
            },
            onEnd: (evt) => {
                // Restore the back of the card after drag
                const listCard = evt.item;
                const cardBack = listCard.querySelector('.list-card-back');
                if (cardBack) {
                    cardBack.style.display = '';
                }
                this.handleListReorder(evt);
            }
        });

        // Store reference globally for flipCard component
        listsSortable = this.listsSortable;
    },

    disableSortable() {
        if (this.listsSortable) {
            this.listsSortable.option("disabled", true);
        }
    },

    enableSortable() {
        if (this.listsSortable) {
            this.listsSortable.option("disabled", false);
        }
    },

    getItemCount(listId) {
        // Access itemsManager to get count of items in this list
        const appEl = document.querySelector('[x-data*="itemsManager"]');
        if (appEl) {
            const itemsManager = Alpine.$data(appEl);
            if (itemsManager && itemsManager.items && itemsManager.items[listId]) {
                return itemsManager.items[listId].length;
            }
        }
        return 0;
    },

    createListElement(list) {
        const div = document.createElement('div');
        div.className = `list-card ${list.collapsed ? 'collapsed' : ''}`;
        div.dataset.listId = list.id;
        div.dataset.flipped = 'false';

        // Mark temp lists
        if (typeof list.id === 'string' && list.id.startsWith('temp-')) {
            div.dataset.isTemp = 'true';
        }

        const colorClass = this.getColorClass(list.color);
        const itemCount = this.getItemCount(list.id);
        const countDisplay = list.collapsed ? ` &mdash; ${itemCount}` : '';

        div.innerHTML = `
            <div class="list-card-inner">
                <div class="list-card-front">
                    <div class="list-header ${colorClass}" data-list-id="${list.id}">
                        <h3>${escapeHtml(list.title)}${countDisplay}</h3>
                        <div class="list-actions">
                            <button class="list-action-btn config-list" title="Configure">⚙️</button>
                        </div>
                    </div>
                    <div class="bookmarks-container" data-list-id="${list.id}"></div>
                    <div class="list-add-buttons">
                        <button class="add-bookmark-btn secondary" data-list-id="${list.id}">+ Link</button>
                        <button class="add-note-btn secondary" data-list-id="${list.id}">+ Note</button>
                    </div>
                </div>
                <div class="list-card-back">
                    <div class="list-config-panel">
                        <div class="list-config-header">
                            <h4>Configure List</h4>
                            <button class="config-close-btn" title="Close">×</button>
                        </div>
                        <div class="config-form-group">
                            <label for="config-list-title-${list.id}">List Name</label>
                            <input type="text" id="config-list-title-${list.id}" name="list-title" class="config-list-title" value="${escapeHtml(list.title)}" autocomplete="off">
                        </div>
                        <div class="config-form-group">
                            <div>Color</div>
                            <div class="config-color-grid" role="radiogroup" aria-label="List color">
                                ${COLORS.map((color, index) => `
                                    <label class="config-color-option ${color.class} ${color.value === list.color ? 'selected' : ''}"
                                           for="color-${list.id}-${index}"
                                           title="${color.name}"> ${color.name}
                                        <input type="radio"
                                                id="color-${list.id}-${index}"
                                                name="color-${list.id}"
                                                value="${color.value}"
                                                data-color="${color.value}"
                                                ${color.value === list.color ? 'checked' : ''}
                                                style="position: absolute; opacity: 0; pointer-events: none;">
                                    </label>
                                `).join('')}
                            </div>
                        </div>
                        <div class="config-actions">
                            <button class="config-delete-btn secondary">Delete List</button>
                            <button class="config-cancel-btn secondary">Cancel</button>
                            <button class="config-save-btn">Save</button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        // Event listeners - List header collapse/expand
        div.querySelector('.list-header').addEventListener('click', (e) => {
            if (div.dataset.flipped === 'true') return;
            if (!e.target.closest('.list-actions')) {
                this.toggleListCollapse(list.id);
            }
        });

        // Gear icon - flip to back
        div.querySelector('.config-list').addEventListener('click', (e) => {
            e.stopPropagation();
            flipToList(list.id);
        });

        // Close button - flip to front
        div.querySelector('.config-close-btn').addEventListener('click', () => {
            closeFlippedCard();
        });

        // Cancel button - flip to front
        div.querySelector('.config-cancel-btn').addEventListener('click', () => {
            closeFlippedCard();
        });

        // Color selection
        div.querySelectorAll('.config-color-option input[type="radio"]').forEach(radio => {
            radio.addEventListener('change', () => {
                div.querySelectorAll('.config-color-option').forEach(o => o.classList.remove('selected'));
                radio.closest('.config-color-option').classList.add('selected');
            });
        });

        // Save button
        div.querySelector('.config-save-btn').addEventListener('click', () => {
            this.saveListConfig(list.id);
        });

        // Delete button
        div.querySelector('.config-delete-btn').addEventListener('click', () => {
            this.deleteListFromConfig(list.id);
        });

        // Enter key to save
        div.querySelector('.config-list-title').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.saveListConfig(list.id);
            }
        });

        // Add bookmark button - dispatch event for items manager
        div.querySelector('.add-bookmark-btn').addEventListener('click', () => {
            const event = new CustomEvent('addBookmarkRequested', { detail: { listId: list.id } });
            document.dispatchEvent(event);
        });

        // Add note button - dispatch event for items manager
        div.querySelector('.add-note-btn').addEventListener('click', () => {
            const event = new CustomEvent('addNoteRequested', { detail: { listId: list.id } });
            document.dispatchEvent(event);
        });

        return div;
    },

    async saveListConfig(listId) {
        const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
        const list = this.lists.find(l => l.id === listId);
        if (!card || !list) return;

        const isTemp = card.dataset.isTemp === 'true';
        const newTitle = card.querySelector('.config-list-title').value.trim();
        const selectedColorRadio = card.querySelector('.config-color-option input[type="radio"]:checked');
        const selectedColor = selectedColorRadio?.value;

        if (isTemp) {
            // Cancel if empty
            if (!newTitle) {
                const index = this.lists.findIndex(l => l.id === listId);
                if (index !== -1) this.lists.splice(index, 1);
                this.renderLists();
                closeFlippedCard();
                return;
            }

            // Create new list
            try {
                const boardId = this.getBoardId();
                if (!boardId) {
                    alert('No board selected');
                    return;
                }

                const createdList = await createList(newTitle, selectedColor, boardId);
                const index = this.lists.findIndex(l => l.id === listId);
                if (index !== -1) this.lists.splice(index, 1);
                this.lists.push(createdList);

                // Update cache
                const event = new CustomEvent('listsUpdated', { detail: { lists: this.lists } });
                document.dispatchEvent(event);

                this.renderLists();
                closeFlippedCard();
            } catch (error) {
                console.error('Failed to create list:', error);
                alert('Failed to create list: ' + error.message);
            }
            return;
        }

        if (!newTitle) {
            alert('List title cannot be empty');
            return;
        }

        try {
            const updates = {};
            if (newTitle !== list.title) updates.title = newTitle;
            if (selectedColor && selectedColor !== list.color) updates.color = selectedColor;

            if (Object.keys(updates).length > 0) {
                await updateList(listId, updates);

                // Update local state
                if (updates.title) list.title = updates.title;
                if (updates.color) list.color = updates.color;

                // Update cache via event
                const event = new CustomEvent('listsUpdated', { detail: { lists: this.lists } });
                document.dispatchEvent(event);

                // Update front of card
                if (updates.title) {
                    card.querySelector('.list-header h3').textContent = updates.title;
                }
                if (updates.color) {
                    const headerEl = card.querySelector('.list-header');
                    COLORS.forEach(c => headerEl.classList.remove(c.class));
                    const colorClass = this.getColorClass(updates.color);
                    headerEl.classList.add(colorClass);
                }
            }

            closeFlippedCard();
        } catch (error) {
            console.error('Failed to save list configuration:', error);
            alert('Failed to save changes');
        }
    },

    async deleteListFromConfig(listId) {
        const list = this.lists.find(l => l.id === listId);
        if (!list) return;

        const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
        const panel = card.querySelector('.list-config-panel');

        // Store original HTML
        const originalHTML = panel.innerHTML;

        // Show confirmation UI
        panel.innerHTML = `
            <div class="list-config-header">
                <h4>Delete List?</h4>
            </div>
            <p style="margin: 1rem 0;">Delete "${escapeHtml(list.title)}"? This will also delete all bookmarks in this list.</p>
            <div class="config-actions">
                <button class="confirm-delete-cancel-btn secondary">Cancel</button>
                <button class="confirm-delete-btn config-delete-btn">Delete</button>
            </div>
        `;

        // Cancel confirmation
        panel.querySelector('.confirm-delete-cancel-btn').addEventListener('click', () => {
            panel.innerHTML = originalHTML;
            closeFlippedCard();
        });

        // Confirm delete
        panel.querySelector('.confirm-delete-btn').addEventListener('click', async () => {
            try {
                await deleteList(listId);
                this.lists = this.lists.filter(l => l.id !== listId);

                // Notify items manager to delete bookmarks
                const event = new CustomEvent('listDeleted', { detail: { listId } });
                document.dispatchEvent(event);

                // Update cache
                const updateEvent = new CustomEvent('listsUpdated', { detail: { lists: this.lists } });
                document.dispatchEvent(updateEvent);

                setCurrentlyFlippedCard(null);
                card.remove();
            } catch (error) {
                console.error('Failed to delete list:', error);
                alert('Failed to delete list');
                panel.innerHTML = originalHTML;
            }
        });
    },

    async toggleListCollapse(listId) {
        const list = this.lists.find(l => l.id === listId);
        if (!list) return;

        list.collapsed = !list.collapsed;

        try {
            await updateList(listId, { collapsed: list.collapsed });
            const listEl = document.querySelector(`.list-card[data-list-id="${listId}"]`);
            listEl.classList.toggle('collapsed', list.collapsed);

            // Update the title to show/hide item count
            const itemCount = this.getItemCount(listId);
            const countDisplay = list.collapsed ? ` (${itemCount})` : '';
            const h3 = listEl.querySelector('.list-header h3');
            h3.textContent = list.title + countDisplay;
        } catch (error) {
            console.error('Failed to toggle list:', error);
            list.collapsed = !list.collapsed;
        }
    },

    async handleListReorder(evt) {
        const listElements = document.querySelectorAll('.list-card');
        const reorderedLists = Array.from(listElements).map((el, index) => ({
            id: parseInt(el.dataset.listId),
            position: index
        }));

        try {
            await reorderLists(reorderedLists);

            // Update local state
            this.lists.forEach(list => {
                const item = reorderedLists.find(r => r.id === list.id);
                if (item) list.position = item.position;
            });
            this.lists.sort((a, b) => a.position - b.position);

            // Update cache via event
            const event = new CustomEvent('listsUpdated', { detail: { lists: this.lists } });
            document.dispatchEvent(event);
        } catch (error) {
            console.error('Failed to reorder lists:', error);
            // Reload on failure
            this.loadData();
        }
    },

    addList() {
        const tempId = `temp-${Date.now()}`;
        const tempList = {
            id: tempId,
            title: '',
            color: COLORS[0].value,
            collapsed: false
        };

        this.lists.push(tempList);
        this.renderLists();

        // Flip to back
        flipToList(tempId);
    },

    getColorClass(colorValue) {
        const color = COLORS.find(c => c.value === colorValue);
        return color ? color.class : 'color-blue';
    }
}));
});
