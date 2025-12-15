// Lists Management Component

import { getLists, createList, updateList, deleteList, reorderLists, copyOrMoveList, debounce, escapeHtml, dispatchEvent } from '../utils/api.js';
import { flipToList, closeFlippedCard } from './flipCard.js';
import { Events } from './events.js';

// Color palette - Darker, more readable colors
const COLORS = [
    { name: 'Blue', value: '#3D6D95' },
    { name: 'Green', value: '#4D7831' },
    { name: 'Orange', value: '#B85720' },
    { name: 'Red', value: '#A43529' },
    { name: 'Purple', value: '#6B3D7D' },
    { name: 'Pink', value: '#924F7D' },
    { name: 'Teal', value: '#358178' },
    { name: 'Gray', value: '#697374' }
];

document.addEventListener('alpine:init', () => {
    Alpine.data('listsManager', () => ({
    lists: [],
    boards: [],
    currentBoardId: null,
    listsSortable: null,
    debouncedListReorder: null,

    init() {
        // Create debounced reorder handler (100ms delay to prevent excessive API calls)
        this.debouncedListReorder = debounce((evt) => {
            this.handleListReorder(evt);
        }, 30);

        // Listen for board data loaded (from boards manager)
        document.addEventListener(Events.BOARD_DATA_LOADED, (event) => {
            this.lists = event.detail.lists || [];
            this.boards = event.detail.boards || [];
            this.currentBoardId = event.detail.board?.id || null;

            // Dispatch items to items manager (support both old 'bookmarks' and new 'items' format)
            dispatchEvent(Events.BOOKMARKS_DATA_LOADED, {
                bookmarks: event.detail.items || event.detail.bookmarks
            });

            this.$nextTick(() => this.renderLists());
        });

        // Listen for user logout event
        document.addEventListener(Events.USER_LOGGED_OUT, () => {
            this.lists = [];
            if (this.listsSortable) {
                this.listsSortable.destroy();
                this.listsSortable = null;
            }
        });

        // Listen for flipCard events to update sortable state
        document.addEventListener(Events.LIST_FLIPPED, () => {
            this.disableSortable();
        });

        // Listen for removeTempList event
        document.addEventListener(Events.REMOVE_TEMP_LIST, (event) => {
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
            dispatchEvent(Events.RENDER_LIST_BOOKMARKS, { listId: list.id });
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
            onEnd: (evt) => {
                // Use debounced handler to prevent excessive API calls during rapid dragging
                this.debouncedListReorder(evt);
            }
        });

        // Store reference in Alpine.store for flipCard component
        Alpine.store('flipCard').setListsSortable(this.listsSortable);
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
        const itemCount = this.getItemCount(list.id);

        div.className = `list-card ${list.collapsed ? 'collapsed' : ''}`;
        div.dataset.listId = list.id;
        div.dataset.flipped = 'false';
        div.dataset.itemCount = itemCount; // Add item count for gradient scaling

        // Mark temp lists
        if (typeof list.id === 'string' && list.id.startsWith('temp-')) {
            div.dataset.isTemp = 'true';
        }

        const countDisplay = list.collapsed ? ` &mdash; ${itemCount}` : '';

        div.innerHTML = `
            <div class="list-card-inner">
                <div class="list-card-front">
                    <div class="list-header" style="background-color: ${list.color};" data-list-id="${list.id}">
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
                <div class="list-card-back" style="display: none;">
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
                            <div class="config-color-presets">
                                ${COLORS.map(c => `
                                    <button type="button"
                                            class="color-preset-btn"
                                            data-color="${c.value}"
                                            style="background-color: ${c.value};"
                                            title="${c.name}"
                                            aria-label="${c.name}">
                                    </button>
                                `).join('')}
                            </div>
                            <div class="config-color-custom">
                                <label for="color-picker-${list.id}">Custom:</label>
                                <input type="color"
                                       class="color-picker-input"
                                       id="color-picker-${list.id}"
                                       value="${list.color}">
                            </div>
                        </div>
                        ${this.boards.length > 1 ? `
                        <div class="config-form-group">
                            <label for="config-board-target-${list.id}">Copy/Move to Board</label>
                            <select id="config-board-target-${list.id}" class="config-board-target">
                                <option value="">Select a board...</option>
                                ${this.boards.filter(b => b.id !== this.currentBoardId).map(board => `
                                    <option value="${board.id}">${escapeHtml(board.title)}</option>
                                `).join('')}
                            </select>
                            <div class="config-board-actions">
                                <button class="config-copy-btn secondary" disabled>Copy to Board</button>
                                <button class="config-move-btn secondary" disabled>Move to Board</button>
                            </div>
                        </div>
                        ` : ''}
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

        // Color preset buttons
        div.querySelectorAll('.color-preset-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const color = btn.dataset.color;
                const colorPicker = div.querySelector('.color-picker-input');
                if (colorPicker) {
                    colorPicker.value = color;
                    // Update header color immediately
                    const header = div.closest('.list').querySelector('.list-header');
                    if (header) {
                        header.style.backgroundColor = color;
                    }
                }
            });
        });

        // Color picker change handler
        const colorPicker = div.querySelector('.color-picker-input');
        if (colorPicker) {
            colorPicker.addEventListener('change', () => {
                const color = colorPicker.value;
                // Update header color immediately
                const header = div.closest('.list').querySelector('.list-header');
                if (header) {
                    header.style.backgroundColor = color;
                }
            });
        }

        // Save button
        div.querySelector('.config-save-btn').addEventListener('click', () => {
            this.saveListConfig(list.id);
        });

        // Delete button
        div.querySelector('.config-delete-btn').addEventListener('click', () => {
            this.deleteListFromConfig(list.id);
        });

        // Board selector - enable/disable copy/move buttons
        const boardSelector = div.querySelector('.config-board-target');
        const copyBtn = div.querySelector('.config-copy-btn');
        const moveBtn = div.querySelector('.config-move-btn');

        if (boardSelector && copyBtn && moveBtn) {
            boardSelector.addEventListener('change', () => {
                const hasSelection = boardSelector.value !== '';
                copyBtn.disabled = !hasSelection;
                moveBtn.disabled = !hasSelection;
            });

            // Copy button
            copyBtn.addEventListener('click', () => {
                this.copyOrMoveListToBoard(list.id, parseInt(boardSelector.value), true);
            });

            // Move button
            moveBtn.addEventListener('click', () => {
                this.copyOrMoveListToBoard(list.id, parseInt(boardSelector.value), false);
            });
        }

        // Enter key to save
        div.querySelector('.config-list-title').addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.saveListConfig(list.id);
            }
        });

        // Add bookmark button - dispatch event for items manager
        div.querySelector('.add-bookmark-btn').addEventListener('click', () => {
            dispatchEvent(Events.ADD_BOOKMARK_REQUESTED, { listId: list.id });
        });

        // Add note button - dispatch event for items manager
        div.querySelector('.add-note-btn').addEventListener('click', () => {
            dispatchEvent(Events.ADD_NOTE_REQUESTED, { listId: list.id });
        });

        return div;
    },

    async saveListConfig(listId) {
        const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
        const list = this.lists.find(l => l.id === listId);
        if (!card || !list) return;

        const isTemp = card.dataset.isTemp === 'true';
        const newTitle = card.querySelector('.config-list-title').value.trim();
        const colorPicker = card.querySelector('.color-picker-input');
        const selectedColor = colorPicker?.value;

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
                dispatchEvent(Events.LISTS_UPDATED, { lists: this.lists });

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
                dispatchEvent(Events.LISTS_UPDATED, { lists: this.lists });

                // Update front of card
                if (updates.title) {
                    card.querySelector('.list-header h3').textContent = updates.title;
                }
                if (updates.color) {
                    const headerEl = card.querySelector('.list-header');
                    headerEl.style.backgroundColor = updates.color;
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
                dispatchEvent(Events.LIST_DELETED, { listId });

                // Update cache
                dispatchEvent(Events.LISTS_UPDATED, { lists: this.lists });

                Alpine.store('flipCard').setCurrentlyFlipped(null);
                card.remove();
            } catch (error) {
                console.error('Failed to delete list:', error);
                alert('Failed to delete list');
                panel.innerHTML = originalHTML;
            }
        });
    },

    async copyOrMoveListToBoard(listId, targetBoardId, copy) {
        const list = this.lists.find(l => l.id === listId);
        if (!list) return;

        const targetBoard = this.boards.find(b => b.id === targetBoardId);
        if (!targetBoard) return;

        const operationType = copy ? 'copy' : 'move';

        try {
            await copyOrMoveList(listId, targetBoardId, copy);

            // Close config panel
            closeFlippedCard();

            // If moving, remove the list from current board
            if (!copy) {
                // Remove from local state
                this.lists = this.lists.filter(l => l.id !== listId);

                // Notify items manager to clean up items for this list
                dispatchEvent(Events.LIST_DELETED, { listId });

                // Update cache
                dispatchEvent(Events.LISTS_UPDATED, { lists: this.lists });

                // Re-render lists
                this.renderLists();
            }
        } catch (error) {
            console.error(`Failed to ${operationType} list:`, error);
            alert(`Failed to ${operationType} list to board`);
        }
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
            const countDisplay = list.collapsed ? ` &mdash; ${itemCount}` : '';
            const h3 = listEl.querySelector('.list-header h3');
            h3.innerHTML = list.title + countDisplay;

            // Update data attribute for gradient scaling
            listEl.dataset.itemCount = itemCount;
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
            dispatchEvent(Events.LISTS_UPDATED, { lists: this.lists });
        } catch (error) {
            console.error('Failed to reorder lists:', error);
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

        // Wait for render then flip to back
        this.$nextTick(() => {
            flipToList(tempId);
        });
    },
}));
});
