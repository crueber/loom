// Boards Component

import { getBoards, getBoardData, createBoard, updateBoard, deleteBoard } from '../utils/api.js';
import { loadFromCache, saveToCache, hasDataChanged } from './cache.js';

document.addEventListener('alpine:init', () => {
    Alpine.data('boardsManager', () => ({
        boards: [],
        currentBoard: { title: 'Loading...' },
        boardSwitcherOpen: false,
        mobileMenuOpen: false,
        showRenameUI: false,
        showDeleteUI: false,
        renameBoardTitle: '',

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

            // Listen for board data loaded - this includes the boards list
            document.addEventListener('boardDataLoaded', (event) => {
                console.log('boardDataLoaded event received:', event.detail);
                if (event.detail.board) {
                    this.currentBoard = event.detail.board;
                }
                // IMPORTANT: Get boards from the freshData that includes boards array
                // This will be set by loadBoard when it gets the API response
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

            if (boardId) {
                // Load specific board (includes boards list)
                await this.loadBoard(boardId);
            } else {
                // Load default board (includes boards list)
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
                    // Update state from cache
                    this.currentBoard = cachedData.board;
                    if (cachedData.boards) {
                        this.boards = cachedData.boards;
                        console.log('Boards loaded from cache:', this.boards);
                    }

                    // Dispatch cached data immediately (includes boards in detail)
                    const cachedEvent = new CustomEvent('boardDataLoaded', {
                        detail: {
                            board: cachedData.board,
                            boards: cachedData.boards,
                            lists: cachedData.lists,
                            items: cachedData.items || cachedData.bookmarks
                        }
                    });
                    document.dispatchEvent(cachedEvent);
                }

                // Step 2: Fetch fresh data from server in background
                const response = await fetch(`/api/boards/${boardId}/data`);
                if (response.status === 404) {
                    window.location.href = '/';
                    return;
                }
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                const freshData = await response.json();

                // Step 3: Update state with fresh data
                this.currentBoard = freshData.board;
                if (freshData.boards) {
                    this.boards = freshData.boards;
                    console.log('Boards updated from server:', this.boards);
                }

                // Step 4: Only re-render if data changed
                if (!cachedData || hasDataChanged(cachedData, freshData)) {
                    // Dispatch fresh data (includes boards in detail)
                    const event = new CustomEvent('boardDataLoaded', {
                        detail: {
                            board: freshData.board,
                            boards: freshData.boards,
                            lists: freshData.lists,
                            items: freshData.items
                        }
                    });
                    document.dispatchEvent(event);

                    // Save to cache for next load
                    saveToCache(freshData);
                }
            } catch (error) {
                console.error('Failed to load board:', error);
                if (!cachedData) {
                    alert('Failed to load board. Redirecting to default.');
                    window.location.href = '/';
                }
            }
        },

        async loadDefaultBoard() {
            // Try loading from cache first
            const cachedData = loadFromCache();
            if (cachedData && cachedData.board && cachedData.board.is_default) {
                // We have default board in cache, use it
                await this.loadBoard(cachedData.board.id);
            } else {
                // No cache - need to get boards list to find default
                await this.loadBoards();
                const defaultBoard = this.boards.find(b => b.is_default);
                if (defaultBoard) {
                    await this.loadBoard(defaultBoard.id);
                }
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

        async finishRename() {
            const newTitle = this.renameBoardTitle.trim();
            if (!newTitle || !this.currentBoard) {
                this.showRenameUI = false;
                this.renameBoardTitle = '';
                return;
            }

            try {
                await updateBoard(this.currentBoard.id, newTitle);

                // Update local state
                this.currentBoard.title = newTitle;
                const board = this.boards.find(b => b.id === this.currentBoard.id);
                if (board) {
                    board.title = newTitle;
                }

                // Update cache
                this.updateCache({ board: this.currentBoard });

                this.showRenameUI = false;
                this.renameBoardTitle = '';
            } catch (error) {
                console.error('Failed to update board title:', error);
                alert('Failed to update board title: ' + error.message);
            }
        },

        async finishDelete() {
            if (!this.currentBoard) return;

            // Prevent deleting the only board
            if (this.boards.length <= 1) {
                alert('Cannot delete the only board');
                this.showDeleteUI = false;
                return;
            }

            try {
                await deleteBoard(this.currentBoard.id);

                // Find another board to redirect to
                const remainingBoard = this.boards.find(b => b.id !== this.currentBoard.id);
                if (remainingBoard) {
                    // Navigate to the remaining board
                    if (remainingBoard.is_default) {
                        window.location.href = '/';
                    } else {
                        window.location.href = `/boards/${remainingBoard.id}`;
                    }
                } else {
                    // Reload to get default board
                    window.location.href = '/';
                }
            } catch (error) {
                console.error('Failed to delete board:', error);
                alert('Failed to delete board: ' + error.message);
                this.showDeleteUI = false;
            }
        },

        async deleteBoard(boardId) {
            const board = this.boards.find(b => b.id === boardId);
            if (!board) return;

            if (board.is_default) {
                alert('Cannot delete the default board');
                return;
            }

            try {
                const response = await fetch(`/api/boards/${boardId}`, {
                    method: 'DELETE'
                });

                if (response.status !== 200) {
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
