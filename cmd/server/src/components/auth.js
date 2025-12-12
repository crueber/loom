// Authentication Component

import { logout, getCurrentUser, dispatchEvent } from '../utils/api.js';
import { Events } from './events.js';

document.addEventListener('alpine:init', () => {
    Alpine.data('authManager', () => ({
        // ============================================================
        // State
        // ============================================================
        currentUser: null,
        showLoginScreen: false,

        // ============================================================
        // Lifecycle
        // ============================================================
        async init() {
            await this.checkAuth();
        },

        // ============================================================
        // Public Methods
        // ============================================================
        async checkAuth() {
            try {
                const user = await getCurrentUser();
                this.currentUser = user;
                this.showLoginScreen = false;

                // Dispatch event to trigger app initialization for existing session
                dispatchEvent(Events.USER_LOGGED_IN, { user });
            } catch (error) {
                // Not logged in - show login screen
                this.currentUser = null;
                this.showLoginScreen = true;
            }
        },

        login() {
            window.location.href = '/auth/login';
        },

        async handleLogout() {
            try {
                await logout();
                this.currentUser = null;

                // Dispatch event to trigger cleanup
                dispatchEvent(Events.USER_LOGGED_OUT);

                // Show login screen
                this.showLoginScreen = true;
            } catch (error) {
                console.error('Logout error:', error);
                alert('Logout failed: ' + error.message);
            }
        }
    }));
});
