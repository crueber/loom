import { createSignal, Show, For, onMount, onCleanup } from 'solid-js';
import { useAuth } from './AuthContext';
import { useBoard } from './BoardContext';
import { useI18n } from './I18nContext';
import { exportData, importData } from '../utils/api';

const LOCALE_FLAGS = {
  'en': '🇺🇸',
  'es': '🇪🇸',
  'fr': '🇫🇷',
  'de': '🇩🇪',
  'pt': '🇵🇹',
  'ru': '🇷🇺',
  'ar': '🇸🇦',
  'zh': '🇨🇳',
  'ja': '🇯🇵',
  'el': '🇬🇷',
  'ga': '🇮🇪',
  'la': '🏛️'
};

export function Navigation() {
  const { user, logout, toggleTheme } = useAuth();
  const { boards, currentBoard, createBoard, updateBoard, deleteBoard } = useBoard();
  const { t } = useI18n();
  
  const [boardSwitcherOpen, setBoardSwitcherOpen] = createSignal(false);
  const [localeSwitcherOpen, setLocaleSwitcherOpen] = createSignal(false);
  const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
  const [showRenameUI, setShowRenameUI] = createSignal(false);
  const [showDeleteUI, setShowDeleteUI] = createSignal(false);
  const [renameBoardTitle, setRenameBoardTitle] = createSignal('');
  const [showImportModal, setShowImportModal] = createSignal(false);
  const [importFile, setImportFile] = createSignal(null);
  const [importMode, setImportMode] = createSignal('merge');
  const [importError, setImportError] = createSignal('');

  let renameInput;
  let renameInputMobile;
  let boardSwitcherRef;
  let localeSwitcherRef;

  const handleClickOutside = (e) => {
    if (boardSwitcherOpen() && boardSwitcherRef && !boardSwitcherRef.contains(e.target)) {
      setBoardSwitcherOpen(false);
      setShowRenameUI(false);
      setShowDeleteUI(false);
    }
    if (localeSwitcherOpen() && localeSwitcherRef && !localeSwitcherRef.contains(e.target)) {
      setLocaleSwitcherOpen(false);
    }
  };

  onMount(() => {
    document.addEventListener('mousedown', handleClickOutside);
  });

  onCleanup(() => {
    document.removeEventListener('mousedown', handleClickOutside);
  });

  const handleRename = (e) => {
    if (e) e.stopPropagation();
    updateBoard(currentBoard.id, renameBoardTitle());
    setShowRenameUI(false);
  };

  const handleShowRename = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setRenameBoardTitle(currentBoard.title);
    setShowRenameUI(true);
    // Use setTimeout to ensure the element is rendered before focusing
    setTimeout(() => {
      if (renameInput) {
        renameInput.focus();
        renameInput.select();
      }
      if (renameInputMobile) {
        renameInputMobile.focus();
        renameInputMobile.select();
      }
    }, 0);
  };

  const handleDelete = (e) => {
    if (e) e.stopPropagation();
    deleteBoard(currentBoard.id);
    setShowDeleteUI(false);
  };

  const handleDeleteUI = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setShowDeleteUI(true);
  };

  const handleImport = async (e) => {
    e.preventDefault();
    setImportError('');
    
    const file = importFile();
    if (!file) {
      setImportError('Please select a file');
      return;
    }

    try {
      const reader = new FileReader();
      reader.onload = async (event) => {
        try {
          const data = JSON.parse(event.target.result);
          await importData(data, importMode());
          window.location.reload();
        } catch (err) {
          setImportError('Invalid JSON file: ' + err.message);
        }
      };
      reader.readAsText(file);
    } catch (err) {
      setImportError('Import failed: ' + err.message);
    }
  };

  return (
    <>
      <nav class="container-fluid">
        <ul>
          <li><strong>Loom</strong></li>
        </ul>
        
        <ul class="nav-desktop">
          <li>
            <div class="nav-user-container">
              <span>{user()?.username}</span>
              <div class="locale-selector" ref={localeSwitcherRef}>
                <button 
                  class="locale-btn"
                  title={t('nav.change_language')}
                  onClick={() => setLocaleSwitcherOpen(!localeSwitcherOpen())}
                >
                  {LOCALE_FLAGS[user()?.locale] || '🌐'}
                </button>
                <Show when={localeSwitcherOpen()}>
                  <div class="locale-dropdown">
                    <For each={Object.entries(LOCALE_FLAGS)}>
                      {([code, flag]) => (
                        <a 
                          href="#" 
                          class={user()?.locale === code ? 'active' : ''}
                          onClick={async (e) => {
                            e.preventDefault();
                            await fetch('/api/user/locale', {
                              method: 'POST',
                              headers: { 'Content-Type': 'application/json' },
                              body: JSON.stringify({ locale: code })
                            });
                            window.location.reload();
                          }}
                        >
                          <span class="locale-flag">{flag}</span>
                          <span class="locale-name">
                            {code === 'en' ? 'English' : 
                             code === 'es' ? 'Español' : 
                             code === 'fr' ? 'Français' : 
                             code === 'de' ? 'Deutsch' : 
                             code === 'pt' ? 'Português' : 
                             code === 'ru' ? 'Русский' : 
                             code === 'ar' ? 'العربية' : 
                             code === 'zh' ? '中文' : 
                             code === 'ja' ? '日本語' : 
                             code === 'el' ? 'Ελληνικά' : 
                             code === 'ga' ? 'Gaeilge' : 
                             code === 'la' ? 'Latin' : code}
                          </span>
                        </a>
                      )}
                    </For>
                  </div>
                </Show>
              </div>
              <button 
                class="theme-toggle-btn" 
                onClick={toggleTheme}
                title={t('nav.toggle_theme')}
              >
                {user()?.theme === 'light' ? '☀️' : '🌙'}
              </button>
            </div>
          </li>
          <li>
            <div class="board-switcher" ref={boardSwitcherRef}>
              <button 
                onClick={() => setBoardSwitcherOpen(!boardSwitcherOpen())} 
                class="board-switcher-btn secondary"
              >
                <span>{currentBoard.title}</span>
                <span class="board-switcher-arrow">▼</span>
              </button>
              
              <Show when={boardSwitcherOpen()}>
                <div class="board-dropdown">
                  <Show when={!showRenameUI() && !showDeleteUI()} fallback={
                    <div class="board-action-panel">
                      <Show when={showRenameUI()}>
                        <label for="rename-board-input">{t('nav.rename_board')}</label>
                        <input 
                          ref={renameInput}
                          id="rename-board-input"
                          type="text" 
                          value={renameBoardTitle()} 
                          onInput={(e) => setRenameBoardTitle(e.currentTarget.value)}
                          onKeyDown={(e) => e.key === 'Enter' && handleRename()}
                          placeholder={t('board.name_placeholder')} 
                        />
                        <div class="board-action-buttons">
                          <button class="secondary" onClick={() => setShowRenameUI(false)}>{t('item.cancel')}</button>
                          <button onClick={handleRename}>{t('item.save')}</button>
                        </div>
                      </Show>
                      <Show when={showDeleteUI()}>
                        <h4>{t('nav.delete_board')}?</h4>
                        <p>{t('nav.delete_board_confirm').replace('{{title}}', currentBoard.title)}</p>
                        <div class="board-action-buttons">
                          <button class="secondary" onClick={() => setShowDeleteUI(false)}>{t('item.cancel')}</button>
                          <button class="contrast" onClick={handleDelete}>{t('item.delete')}</button>
                        </div>
                      </Show>
                    </div>
                  }>
                    <For each={boards}>
                      {(board) => (
                        <a href={board.is_default ? '/' : `/boards/${board.id}`}>
                          {board.is_default ? '። ' : ''}{board.title}
                        </a>
                      )}
                    </For>
                    <hr />
                    <a href="#" onClick={handleShowRename}>{t('nav.rename_board')}</a>
                    <Show when={!currentBoard.is_default}>
                      <a href="#" onClick={handleDeleteUI}>{t('nav.delete_board')}</a>
                    </Show>
                    <hr />
                    <a href="#" onClick={(e) => { e.preventDefault(); createBoard(); }}>+ {t('nav.new_board')}</a>
                  </Show>
                </div>
              </Show>
            </div>
          </li>
          <li><button class="secondary" onClick={exportData}>{t('nav.export')}</button></li>
          <li><button class="secondary" onClick={() => setShowImportModal(true)}>{t('nav.import')}</button></li>
          <li><button class="contrast" onClick={logout}>{t('nav.logout')}</button></li>
        </ul>

        <ul class="nav-mobile">
          <li>
            <button class="hamburger-btn secondary" onClick={() => setMobileMenuOpen(true)}>
              <span class="hamburger-icon">☰</span>
            </button>
          </li>
        </ul>

        <Show when={mobileMenuOpen()}>
          <div class="mobile-menu-overlay" onClick={() => setMobileMenuOpen(false)}>
            <div class="mobile-menu" onClick={(e) => e.stopPropagation()}>
              <button class="mobile-menu-close" onClick={() => setMobileMenuOpen(false)}>
                <span style="padding: 0 0.5rem;">x</span>
              </button>
              <div class="mobile-menu-section">
                <h3>{t('nav.boards')}</h3>
                <div class="board-switcher-mobile" ref={boardSwitcherRef}>
                  <button 
                    onClick={() => setBoardSwitcherOpen(!boardSwitcherOpen())} 
                    class="board-switcher-btn-mobile secondary"
                  >
                    <span>{currentBoard.title}</span>
                    <span class="board-switcher-arrow">▼</span>
                  </button>
                  
                  <Show when={boardSwitcherOpen()}>
                    <div class="board-dropdown-mobile">
                      <Show when={!showRenameUI() && !showDeleteUI()} fallback={
                        <div class="board-action-panel">
                          <Show when={showRenameUI()}>
                            <label for="rename-board-input-mobile">{t('nav.rename_board')}</label>
                            <input 
                              ref={renameInputMobile}
                              id="rename-board-input-mobile"
                              type="text" 
                              value={renameBoardTitle()} 
                              onInput={(e) => setRenameBoardTitle(e.currentTarget.value)}
                              onKeyDown={(e) => e.key === 'Enter' && handleRename()}
                              placeholder={t('board.name_placeholder')} 
                            />
                            <div class="board-action-buttons">
                              <button class="secondary" onClick={() => setShowRenameUI(false)}>{t('item.cancel')}</button>
                              <button onClick={handleRename}>{t('item.save')}</button>
                            </div>
                          </Show>
                          <Show when={showDeleteUI()}>
                            <h4>{t('nav.delete_board')}?</h4>
                            <p>{t('nav.delete_board_confirm').replace('{{title}}', currentBoard.title)}</p>
                            <div class="board-action-buttons">
                              <button class="secondary" onClick={() => setShowDeleteUI(false)}>{t('item.cancel')}</button>
                              <button class="contrast" onClick={handleDelete}>{t('item.delete')}</button>
                            </div>
                          </Show>
                        </div>
                      }>
                        <For each={boards}>
                          {(board) => (
                            <a href={board.is_default ? '/' : `/boards/${board.id}`}>
                              {board.is_default ? '። ' : ''}{board.title}
                            </a>
                          )}
                        </For>
                        <hr />
                        <a href="#" onClick={handleShowRename}>{t('nav.rename_board')}</a>
                        <Show when={!currentBoard.is_default}>
                          <a href="#" onClick={handleDeleteUI}>{t('nav.delete_board')}</a>
                        </Show>
                        <hr />
                        <a href="#" onClick={(e) => { e.preventDefault(); createBoard(); }}>+ {t('nav.new_board')}</a>
                      </Show>
                    </div>
                  </Show>
                </div>
              </div>
              <div class="mobile-menu-section">
                <h3>{t('nav.settings')}</h3>
                <div class="nav-user-container" style={{ "margin-bottom": '1rem' }}>
                  <strong>{user()?.username}</strong>
                  <div class="locale-selector" ref={localeSwitcherRef}>
                    <button 
                      class="locale-btn"
                      title={t('nav.change_language')}
                      onClick={() => setLocaleSwitcherOpen(!localeSwitcherOpen())}
                    >
                      {LOCALE_FLAGS[user()?.locale] || '🌐'}
                    </button>
                    <Show when={localeSwitcherOpen()}>
                      <div class="locale-dropdown-mobile">
                        <For each={Object.entries(LOCALE_FLAGS)}>
                          {([code, flag]) => (
                            <a 
                              href="#" 
                              class={user()?.locale === code ? 'active' : ''}
                              onClick={async (e) => {
                                e.preventDefault();
                                await fetch('/api/user/locale', {
                                  method: 'POST',
                                  headers: { 'Content-Type': 'application/json' },
                                  body: JSON.stringify({ locale: code })
                                });
                                window.location.reload();
                              }}
                            >
                              <span class="locale-flag">{flag}</span>
                              <span class="locale-name">
                                {code === 'en' ? 'English' : 
                                 code === 'es' ? 'Español' : 
                                 code === 'fr' ? 'Français' : 
                                 code === 'de' ? 'Deutsch' : 
                                 code === 'pt' ? 'Português' : 
                                 code === 'ru' ? 'Русский' : 
                                 code === 'ar' ? 'العربية' : 
                                 code === 'zh' ? '中文' : 
                                 code === 'ja' ? '日本語' : 
                                 code === 'el' ? 'Ελληνικά' : 
                                 code === 'ga' ? 'Gaeilge' : 
                                 code === 'la' ? 'Latin' : code}
                              </span>
                            </a>
                          )}
                        </For>
                      </div>
                    </Show>
                  </div>
                  <button 
                    class="theme-toggle-btn" 
                    onClick={toggleTheme}
                    title={t('nav.toggle_theme')}
                  >
                    {user()?.theme === 'light' ? '☀️' : '🌙'}
                  </button>
                </div>
                <button class="secondary mobile-menu-btn" onClick={exportData}>{t('nav.export')}</button>
                <button class="secondary mobile-menu-btn" onClick={() => setShowImportModal(true)}>{t('nav.import')}</button>
                <button class="contrast mobile-menu-btn" onClick={logout}>{t('nav.logout')}</button>
              </div>
            </div>
          </div>
        </Show>
      </nav>

      <Show when={showImportModal()}>
        <dialog open>
          <article>
            <header>
              <button aria-label="Close" rel="prev" onClick={() => setShowImportModal(false)}></button>
              <h3>{t('nav.import_modal_title')}</h3>
            </header>
            <form onSubmit={handleImport}>
              <div class="config-form-group">
                <label for="import-file-input">{t('nav.import_file_label')}</label>
                <input 
                  id="import-file-input"
                  type="file" 
                  accept=".json" 
                  onChange={(e) => setImportFile(e.currentTarget.files[0])} 
                  required 
                />
              </div>
              <div class="config-form-group">
                <label for="import-mode-select">{t('nav.import_mode_label')}</label>
                <select id="import-mode-select" value={importMode()} onChange={(e) => setImportMode(e.currentTarget.value)}>
                  <option value="merge">{t('nav.import_mode_merge')}</option>
                  <option value="replace">{t('nav.import_mode_replace')}</option>
                </select>
              </div>
              <Show when={importError()}>
                <p class="error">{importError()}</p>
              </Show>
              <footer>
                <button type="button" class="secondary" onClick={() => setShowImportModal(false)}>{t('item.cancel')}</button>
                <button type="submit">{t('nav.import')}</button>
              </footer>
            </form>
          </article>
        </dialog>
      </Show>
    </>
  );
}
