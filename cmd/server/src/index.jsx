import { render } from 'solid-js/web';
import { AuthProvider, useAuth } from './components/AuthContext';
import { BoardProvider, useBoard } from './components/BoardContext';
import { I18nProvider, useI18n } from './components/I18nContext';
import { Navigation } from './components/Navigation';
import { ListsManager } from './components/ListsManager';
import { Show, createEffect } from 'solid-js';

function LoginScreen() {
  const { login } = useAuth();
  const { t } = useI18n();
  return (
    <div id="login-screen" class="container">
      <main>
        <article>
          <header>
            <h1>{t('app.name')}</h1>
            <p>{t('app.tagline')}</p>
          </header>
          <div style="text-align: center; padding: 2rem;">
            <button onClick={login}>{t('auth.login_oauth')}</button>
          </div>
        </article>
      </main>
    </div>
  );
}

function App() {
  const { showLoginScreen } = useAuth();
  const { currentBoard } = useBoard();

  // Update document title when board changes
  createEffect(() => {
    const title = currentBoard.title === 'Loading...' ? 'Loom' : `${currentBoard.title} | Loom`;
    document.title = title;
  });
  
  return (
    <Show when={!showLoginScreen()} fallback={<LoginScreen />}>
      <div id="app-screen">
        <Navigation />
        <ListsManager />
      </div>
    </Show>
  );
}

const root = document.getElementById('app-mount');
if (root) {
  render(() => (
    <I18nProvider>
      <AuthProvider>
        <BoardProvider>
          <App />
        </BoardProvider>
      </AuthProvider>
    </I18nProvider>
  ), root);

  // Clear bootstrap data after initialization to prevent reuse
  delete window.__BOOTSTRAP_DATA__;
  delete window.__I18N_DATA__;
}
