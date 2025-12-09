// Authentication Component

import { login, logout, getCurrentUser } from '../utils/api.js';

document.addEventListener('alpine:init', () => {
    Alpine.data('authManager', () => ({
    currentUser: null,
    showLoginScreen: false,
    loginError: '',

    init() {
        this.checkAuth();
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
            // Not logged in
            this.currentUser = null;
            this.showLoginScreen = true;
        }
    },

    async handleLogin(event) {
        event.preventDefault();
        this.loginError = '';

        const form = event.target;
        const username = form.querySelector('#login-username').value.trim();
        const password = form.querySelector('#login-password').value;

        if (!username || !password) {
            this.loginError = 'Username and password required';
            return;
        }

        try {
            const result = await login(username, password);
            this.currentUser = result.user;
            this.showLoginScreen = false;

            // Dispatch event to trigger app initialization
            const loginEvent = new CustomEvent('userLoggedIn', { detail: { user: result.user } });
            document.dispatchEvent(loginEvent);
        } catch (error) {
            this.loginError = error.message || 'Login failed';
            console.error('Login error:', error);
        }
    },

    async handleLogout() {
        try {
            await logout();
            this.currentUser = null;
            this.showLoginScreen = true;

            // Dispatch event to trigger cleanup
            const logoutEvent = new CustomEvent('userLoggedOut');
            document.dispatchEvent(logoutEvent);
        } catch (error) {
            console.error('Logout error:', error);
            alert('Logout failed: ' + error.message);
        }
    }
}));
});
