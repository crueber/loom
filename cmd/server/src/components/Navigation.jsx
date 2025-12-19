import { createSignal, Show, For } from 'solid-js';
import { useAuth } from './AuthContext';
import { useBoard } from './BoardContext';
import { exportData, importData } from '../utils/api';

export function Navigation() {
  const { user, logout } = useAuth();
  const { boards, currentBoard, createBoard, updateBoard, deleteBoard } = useBoard();
  
  const [boardSwitcherOpen, setBoardSwitcherOpen] = createSignal(false);
  const [mobileMenuOpen, setMobileMenuOpen] = createSignal(false);
  const [showRenameUI, setShowRenameUI] = createSignal(false);
  const [showDeleteUI, setShowDeleteUI] = createSignal(false);
  const [renameBoardTitle, setRenameBoardTitle] = createSignal('');
  const [showImportModal, setShowImportModal] = createSignal(false);
  const [importFile, setImportFile] = createSignal(null);
  const [importMode, setImportMode] = createSignal('merge');
  const [importError, setImportError] = createSignal('');

  const handleRename = () => {
    updateBoard(currentBoard.id, renameBoardTitle());
    setShowRenameUI(false);
  };

  const handleDelete = () => {
    deleteBoard(currentBoard.id);
    setShowDeleteUI(false);
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
          <li><span style={{ "margin-right": "1rem" }}>{user()?.username}</span></li>
          <li>
            <div class="board-switcher">
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
                        <label for="rename-board-input">Rename Board</label>
                        <input 
                          id="rename-board-input"
                          type="text" 
                          value={renameBoardTitle()} 
                          onInput={(e) => setRenameBoardTitle(e.currentTarget.value)}
                          onKeyDown={(e) => e.key === 'Enter' && handleRename()}
                          placeholder="Board name" 
                        />
                        <div class="board-action-buttons">
                          <button class="secondary" onClick={() => setShowRenameUI(false)}>Cancel</button>
                          <button onClick={handleRename}>Save</button>
                        </div>
                      </Show>
                      <Show when={showDeleteUI()}>
                        <h4>Delete Board?</h4>
                        <p>Delete '{currentBoard.title}'? This will also delete all lists and items in this board.</p>
                        <div class="board-action-buttons">
                          <button class="secondary" onClick={() => setShowDeleteUI(false)}>Cancel</button>
                          <button class="contrast" onClick={handleDelete}>Delete</button>
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
                    <a href="#" onClick={(e) => { e.preventDefault(); setRenameBoardTitle(currentBoard.title); setShowRenameUI(true); }}>Rename Board</a>
                    <Show when={!currentBoard.is_default}>
                      <a href="#" onClick={(e) => { e.preventDefault(); setShowDeleteUI(true); }}>Delete Board</a>
                    </Show>
                    <hr />
                    <a href="#" onClick={(e) => { e.preventDefault(); createBoard(); }}>+ New Board</a>
                  </Show>
                </div>
              </Show>
            </div>
          </li>
          <li><button class="secondary" onClick={exportData}>Export</button></li>
          <li><button class="secondary" onClick={() => setShowImportModal(true)}>Import</button></li>
          <li><button class="contrast" onClick={logout}>Logout</button></li>
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
                <h3>Boards</h3>
                <div class="board-switcher-mobile">
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
                            <label for="rename-board-input-mobile">Rename Board</label>
                            <input 
                              id="rename-board-input-mobile"
                              type="text" 
                              value={renameBoardTitle()} 
                              onInput={(e) => setRenameBoardTitle(e.currentTarget.value)}
                              onKeyDown={(e) => e.key === 'Enter' && handleRename()}
                              placeholder="Board name" 
                            />
                            <div class="board-action-buttons">
                              <button class="secondary" onClick={() => setShowRenameUI(false)}>Cancel</button>
                              <button onClick={handleRename}>Save</button>
                            </div>
                          </Show>
                          <Show when={showDeleteUI()}>
                            <h4>Delete Board?</h4>
                            <p>Delete '{currentBoard.title}'? This will also delete all lists and items in this board.</p>
                            <div class="board-action-buttons">
                              <button class="secondary" onClick={() => setShowDeleteUI(false)}>Cancel</button>
                              <button class="contrast" onClick={handleDelete}>Delete</button>
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
                        <a href="#" onClick={(e) => { e.preventDefault(); setRenameBoardTitle(currentBoard.title); setShowRenameUI(true); }}>Rename Board</a>
                        <Show when={!currentBoard.is_default}>
                          <a href="#" onClick={(e) => { e.preventDefault(); setShowDeleteUI(true); }}>Delete Board</a>
                        </Show>
                        <hr />
                        <a href="#" onClick={(e) => { e.preventDefault(); createBoard(); }}>+ New Board</a>
                      </Show>
                    </div>
                  </Show>
                </div>
              </div>
              <div class="mobile-menu-section">
                <h3>Account</h3>
                <p><strong>{user()?.username}</strong></p>
                <button class="secondary mobile-menu-btn" onClick={exportData}>Export</button>
                <button class="secondary mobile-menu-btn" onClick={() => setShowImportModal(true)}>Import</button>
                <button class="contrast mobile-menu-btn" onClick={logout}>Logout</button>
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
              <h3>Import Data</h3>
            </header>
            <form onSubmit={handleImport}>
              <div class="config-form-group">
                <label for="import-file-input">Select JSON file</label>
                <input 
                  id="import-file-input"
                  type="file" 
                  accept=".json" 
                  onChange={(e) => setImportFile(e.currentTarget.files[0])} 
                  required 
                />
              </div>
              <div class="config-form-group">
                <label for="import-mode-select">Import mode</label>
                <select id="import-mode-select" value={importMode()} onChange={(e) => setImportMode(e.currentTarget.value)}>
                  <option value="merge">Merge with existing data</option>
                  <option value="replace">Replace all data</option>
                </select>
              </div>
              <Show when={importError()}>
                <p class="error">{importError()}</p>
              </Show>
              <footer>
                <button type="button" class="secondary" onClick={() => setShowImportModal(false)}>Cancel</button>
                <button type="submit">Import</button>
              </footer>
            </form>
          </article>
        </dialog>
      </Show>
    </>
  );
}
