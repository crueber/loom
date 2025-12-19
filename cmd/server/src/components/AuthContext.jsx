import { createSignal, createContext, useContext } from 'solid-js';
import { logout as apiLogout } from '../utils/api';

const AuthContext = createContext();

export function AuthProvider(props) {
  const bootstrapData = window.__BOOTSTRAP_DATA__ || {};
  const [user, setUser] = createSignal(bootstrapData.user || null);
  const [showLoginScreen, setShowLoginScreen] = createSignal(!bootstrapData.user);

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
    setUser
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
