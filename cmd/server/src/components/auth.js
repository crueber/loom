// Authentication Component

import { logout, dispatchEvent } from '../utils/api.js';
import { Events } from './events.js';

document.addEventListener('alpine:init', () => {
    Alpine.data('authManager', () => ({
        // ============================================================
        // State
        // ============================================================
        currentUser: null,
        showLoginScreen: true,  // Start with login screen visible

        // ============================================================
        // Lifecycle
        // ============================================================
        init() {
            // Listen for user logged in event from dataBootstrap
            document.addEventListener(Events.USER_LOGGED_IN, (event) => {
                this.currentUser = event.detail.user;
                this.showLoginScreen = false;
            });

            // Listen for user logged out event
            document.addEventListener(Events.USER_LOGGED_OUT, () => {
                this.currentUser = null;
                this.showLoginScreen = true;
            });
        },

        // ============================================================
        // Public Methods
        // ============================================================

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
