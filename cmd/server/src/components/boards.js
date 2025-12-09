// Boards Component

import { getBoards, getBoardData, createBoard, updateBoard, deleteBoard } from '../utils/api.js';
import { loadFromCache, saveToCache } from './cache.js';

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
                if (event.detail.boards) {
                    this.boards = event.detail.boards;
                }
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
