import { createSignal, createContext, useContext } from 'solid-js';
import { logout as apiLogout } from '../utils/api';

const AuthContext = createContext();

export function AuthProvider(props) {
  const bootstrapData = window.__BOOTSTRAP_DATA__ || {};
  const [user, setUser] = createSignal(bootstrapData.user || null);
  const [showLoginScreen, setShowLoginScreen] = createSignal(!bootstrapData.user);

  const toggleTheme = async () => {
    const currentUser = user();
    if (!currentUser) return;

    const newTheme = currentUser.theme === 'light' ? 'dark' : 'light';
    
    try {
      const response = await fetch('/api/user/theme', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ theme: newTheme })
      });

      if (response.ok) {
        setUser({ ...currentUser, theme: newTheme });
        document.documentElement.setAttribute('data-theme', newTheme);
      }
    } catch (error) {
      console.error('Failed to update theme:', error);
    }
  };

  const login = () => {
    window.location.href = '/auth/login';
  };

  const logout = async () => {
    try {
      await apiLogout();
      setUser(null);
      setShowLoginScreen(true);
    } catch (error) {
      console.error('Logout error:', error);
      alert('Logout failed: ' + error.message);
    }
  };

  const auth = {
    user,
    showLoginScreen,
    login,
    logout,
    setUser,
    toggleTheme
  };

  return (
    <AuthContext.Provider value={auth}>
      {props.children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
