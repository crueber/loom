// Authentication Component

import { logout, getCurrentUser } from '../utils/api.js';

document.addEventListener('alpine:init', () => {
    Alpine.data('authManager', () => ({
    currentUser: null,
    showLoginScreen: false,

    async init() {
        await this.checkAuth();
    },

    async checkAuth() {
        try {
            const user = await getCurrentUser();
            this.currentUser = user;
            this.showLoginScreen = false;

            // Dispatch event to trigger app initialization for existing session
            const loginEvent = new CustomEvent('userLoggedIn', { detail: { user: user } });
            document.dispatchEvent(loginEvent);
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
            const logoutEvent = new CustomEvent('userLoggedOut');
            document.dispatchEvent(logoutEvent);

            // Show login screen
            this.showLoginScreen = true;
        } catch (error) {
            console.error('Logout error:', error);
            alert('Logout failed: ' + error.message);
        }
    }
}));
});
