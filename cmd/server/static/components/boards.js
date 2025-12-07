// Boards Component

document.addEventListener('alpine:init', () => {
    Alpine.data('boardsManager', () => ({
        boards: [],
        currentBoard: { title: 'Loading...' },
        boardSwitcherOpen: false,
        editingBoardTitle: false,
        editingBoardId: null,

        init() {
            // Listen for user login to load boards
            document.addEventListener('userLoggedIn', () => {
                this.detectCurrentBoard();
            });

            // Listen for logout to clear boards
            document.addEventListener('userLoggedOut', () => {
                this.boards = [];
                this.currentBoard = { title: 'Loading...' };
            });

            // Listen for list/bookmark updates to update cache
            document.addEventListener('listsUpdated', (event) => {
                this.updateCache({ lists: event.detail.lists });
            });

            document.addEventListener('bookmarksUpdated', (event) => {
                this.updateCache({ bookmarks: event.detail.bookmarks });
            });
        },

        updateCache(updates) {
            // Load current cache
            const cachedData = loadFromCache();
            if (cachedData) {
                // Update the cache with new data
                const updatedCache = {
                    ...cachedData,
                    ...updates
                };
                saveToCache(updatedCache);
            }
        },

        async detectCurrentBoard() {
            // Parse URL to determine which board to load
            const path = window.location.pathname;
            const match = path.match(/^\/boards\/(\d+)$/);
            const boardId = match ? parseInt(match[1]) : null;

            // Load all boards for the switcher
            await this.loadBoards();

            if (boardId) {
                // Load specific board
                await this.loadBoard(boardId);
            } else {
                // Load default board
                await this.loadDefaultBoard();
            }
        },

        async loadBoards() {
            try {
                const response = await fetch('/api/boards');
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                this.boards = await response.json();
            } catch (error) {
                console.error('Failed to load boards:', error);
            }
        },

        async loadBoard(boardId) {
            try {
                // Step 1: Try to load from cache first for instant display
                const cachedData = loadFromCache();
                if (cachedData && cachedData.board && cachedData.board.id === boardId) {
                    this.currentBoard = cachedData.board;

                    // Dispatch cached data immediately
                    const cachedEvent = new CustomEvent('boardDataLoaded', {
                        detail: {
                            lists: cachedData.lists,
                            bookmarks: cachedData.bookmarks
                        }
                    });
                    document.dispatchEvent(cachedEvent);
                }

                // Step 2: Fetch fresh data from server in background
                const response = await fetch(`/api/boards/${boardId}/data`);
                if (response.status === 404) {
                    // Board not found, redirect to default
                    window.location.href = '/';
                    return;
                }
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                const freshData = await response.json();

                // Step 3: Only update if data changed or no cache existed
                if (!cachedData || hasDataChanged(cachedData, freshData)) {
                    this.currentBoard = freshData.board;

                    // Dispatch fresh data
                    const event = new CustomEvent('boardDataLoaded', {
                        detail: {
                            lists: freshData.lists,
                            bookmarks: freshData.bookmarks
                        }
                    });
                    document.dispatchEvent(event);

                    // Save to cache for next load
                    saveToCache(freshData);
                }
            } catch (error) {
                console.error('Failed to load board:', error);
                // Only alert if we didn't have cached data
                if (!cachedData) {
                    alert('Failed to load board. Redirecting to default.');
                    window.location.href = '/';
                }
            }
        },

        async loadDefaultBoard() {
            // Find default board from boards list
            let defaultBoard = this.boards.find(b => b.is_default);

            // If no default board exists yet, get it from API
            if (!defaultBoard && this.boards.length === 0) {
                await this.loadBoards();
                defaultBoard = this.boards.find(b => b.is_default);
            }

            if (defaultBoard) {
                this.currentBoard = defaultBoard;
                // Load data for default board
                await this.loadBoard(defaultBoard.id);
            }
        },

        async createBoard() {
            try {
                const response = await fetch('/api/boards', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ title: 'New Board' })
                });

                if (!response.ok) {
                    throw new Error('Failed to create board');
                }

                const newBoard = await response.json();

                // Reload boards list
                await this.loadBoards();

                // Navigate to new board
                window.location.href = `/boards/${newBoard.id}`;
            } catch (error) {
                console.error('Failed to create board:', error);
                alert('Failed to create board: ' + error.message);
            }
        },

        switchBoard(boardId) {
            // Server-side navigation
            if (boardId === this.currentBoard?.id) {
                return; // Already on this board
            }

            const board = this.boards.find(b => b.id === boardId);
            if (board && board.is_default) {
                window.location.href = '/';
            } else {
                window.location.href = `/boards/${boardId}`;
            }
        },

        startEditingTitle() {
            if (this.currentBoard) {
                this.editingBoardTitle = true;
                this.editingBoardId = this.currentBoard.id;
                // Focus the input on next tick
                this.$nextTick(() => {
                    const input = document.getElementById('board-title-input');
                    if (input) {
                        input.focus();
                        input.select();
                    }
                });
            }
        },

        async saveTitle(event) {
            if (event.key === 'Enter') {
                event.preventDefault();
                await this.finishEditingTitle();
            } else if (event.key === 'Escape') {
                this.cancelEditingTitle();
            }
        },

        async finishEditingTitle() {
            const input = document.getElementById('board-title-input');
            if (!input) return;

            const newTitle = input.value.trim();
            if (!newTitle) {
                this.cancelEditingTitle();
                return;
            }

            try {
                const response = await fetch(`/api/boards/${this.editingBoardId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ title: newTitle })
                });

                if (!response.ok) {
                    throw new Error('Failed to update board title');
                }

                // Update local state
                this.currentBoard.title = newTitle;
                const board = this.boards.find(b => b.id === this.editingBoardId);
                if (board) {
                    board.title = newTitle;
                }

                // Update cache
                this.updateCache({ board: this.currentBoard });

                this.editingBoardTitle = false;
                this.editingBoardId = null;
            } catch (error) {
                console.error('Failed to update board title:', error);
                alert('Failed to update board title: ' + error.message);
                this.cancelEditingTitle();
            }
        },

        cancelEditingTitle() {
            this.editingBoardTitle = false;
            this.editingBoardId = null;
        },

        async deleteBoard(boardId) {
            const board = this.boards.find(b => b.id === boardId);
            if (!board) return;

            if (board.is_default) {
                alert('Cannot delete the default board');
                return;
            }

            if (!confirm(`Are you sure you want to delete "${board.title}"? All lists and bookmarks in this board will be deleted.`)) {
                return;
            }

            try {
                const response = await fetch(`/api/boards/${boardId}`, {
                    method: 'DELETE'
                });

                if (!response.ok) {
                    const error = await response.text();
                    throw new Error(error || 'Failed to delete board');
                }

                // If we deleted the current board, redirect to default
                if (boardId === this.currentBoard?.id) {
                    window.location.href = '/';
                } else {
                    // Just reload boards list
                    await this.loadBoards();
                }
            } catch (error) {
                console.error('Failed to delete board:', error);
                alert('Failed to delete board: ' + error.message);
            }
        }
    }));
});
