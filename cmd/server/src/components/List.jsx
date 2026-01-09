import { For, Show, createSignal, onMount, createEffect } from 'solid-js';
import { Item } from './Item';
import { useBoard } from './BoardContext';
import { useI18n } from './I18nContext';
import { ColorPicker } from './ColorPicker';
import Sortable from 'sortablejs';

export function List(props) {
  const { items, updateItem, addItem, saveNewItem, deleteItem, deleteList, updateList, moveItem, reorderLists, lists, boards, currentBoard, copyOrMoveList } = useBoard();
  const { t } = useI18n();
  const [isFlipped, setIsFlipped] = createSignal(props.list.id.toString().startsWith('temp-'));
  const [isCollapsed, setIsCollapsed] = createSignal(props.list.collapsed);
  const [title, setTitle] = createSignal(props.list.title);
  const [color, setColor] = createSignal(props.list.color);
  const [targetBoardId, setTargetBoardId] = createSignal(0);
  const [itemsRef, setItemsRef] = createSignal();
  
  let titleInputRef;

  createEffect(() => {
    if (isFlipped() && titleInputRef) {
      titleInputRef.focus();
      titleInputRef.select();
    }
  });

  const listItems = () => items[props.list.id] || [];

  const getContrastColor = (hexcolor) => {
    if (!hexcolor) return '#ffffff';
    if (!hexcolor.startsWith('#')) return '#ffffff';
    const r = parseInt(hexcolor.substr(1, 2), 16);
    const g = parseInt(hexcolor.substr(3, 2), 16);
    const b = parseInt(hexcolor.substr(5, 2), 16);
    const yiq = ((r * 299) + (g * 587) + (b * 114)) / 1000;
    return (yiq >= 128) ? '#111111' : '#f8f8f8';
  };

  const handleUpdateItem = (itemId, updates) => {
    updateItem(itemId, props.list.id, updates);
  };

  const handleSaveNewItem = (tempId, updates) => {
    const item = listItems().find(i => i.id === tempId);
    if (item) {
      saveNewItem(tempId, props.list.id, item.type, updates);
    }
  };

  createEffect(() => {
    const el = itemsRef();
    if (el && !isCollapsed() && !props.list.id.toString().startsWith('temp-')) {
      const sortable = Sortable.create(el, {
        group: 'items',
        animation: 150,
        ghostClass: 'sortable-ghost',
        chosenClass: 'sortable-chosen',
        dragClass: 'sortable-drag',
        delay: 150,
        delayOnTouchOnly: true,
        filter: '[data-flipped="true"], input, textarea, select, button',
        preventOnFilter: false,
        onEnd: (evt) => {
          const { item, from, to, oldIndex, newIndex } = evt;
          const itemId = parseInt(item.getAttribute('data-id'));
          const fromListId = parseInt(from.getAttribute('data-list-id'));
          const toListId = parseInt(to.getAttribute('data-list-id'));

          if (fromListId !== toListId || oldIndex !== newIndex) {
            // Revert Sortable's DOM change so SolidJS can handle it
            item.remove();
            if (oldIndex >= from.children.length) {
              from.appendChild(item);
            } else {
              from.insertBefore(item, from.children[oldIndex]);
            }
            
            moveItem(itemId, fromListId, toListId, newIndex);
          }
        },
      });

      return () => sortable.destroy();
    }
  });

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSave();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      handleCancel();
    }
  };

  const handleSave = () => {
    if (props.list.id.toString().startsWith('temp-')) {
      props.onSaveNew(props.list.id, { title: title(), color: color() });
    } else {
      updateList(props.list.id, { title: title(), color: color() });
      setIsFlipped(false);
    }
  };

  const handleCancel = () => {
    if (props.list.id.toString().startsWith('temp-')) {
      props.onDelete();
    } else {
      setIsFlipped(false);
      setTitle(props.list.title);
      setColor(props.list.color);
    }
  };

  const handleToggleCollapse = () => {
    if (props.list.id.toString().startsWith('temp-')) return;
    const newState = !isCollapsed();
    setIsCollapsed(newState);
    updateList(props.list.id, { collapsed: newState });
  };

  const handleAddLink = () => {
    addItem(props.list.id, 'bookmark', { title: '', url: 'https://' });
  };

  const handleAddNote = () => {
    addItem(props.list.id, 'note', { title: '', content: '' });
  };

  return (
    <div 
      class="list-card" 
      classList={{ collapsed: isCollapsed() }}
      style={{ "--list-color": color() }}
      data-id={props.list.id}
      data-flipped={isFlipped()}
      data-item-count={listItems().length}
    >
      <div class="list-card-inner">
        {/* Front Side */}
        <div class="list-card-front">
          <header 
            class="list-header" 
            style={{ 
              background: isCollapsed() ? 'none' : color()
            }} 
            onClick={handleToggleCollapse}
          >
            <h3 
              class="list-title"
              style={{ color: isCollapsed() ? 'inherit' : getContrastColor(color()) }}
            >
              {props.list.title}
              <Show when={isCollapsed()}>
                {" — "}{listItems().length}
              </Show>
            </h3>
            <div class="list-actions" onClick={(e) => e.stopPropagation()}>
              <Show when={!isCollapsed()}>
                <button 
                  class="list-action-btn gear-icon" 
                  style={{ color: getContrastColor(color()) }}
                  onClick={() => setIsFlipped(true)}
                >
                  ⚙️
                </button>
              </Show>
            </div>
          </header>
          
          <Show when={!isCollapsed()}>
            <div class="bookmarks-container" ref={setItemsRef} data-list-id={props.list.id}>
              <For each={listItems()}>
                {(item) => (
                  <Item 
                    item={item} 
                    onUpdate={handleUpdateItem} 
                    onSaveNew={handleSaveNewItem}
                    onDelete={() => deleteItem(item.id, props.list.id)} 
                  />
                )}
              </For>
            </div>
            <div class="list-add-buttons">
              <button class="add-link-btn" onClick={handleAddLink}>+ {t('list.add_link')}</button>
              <button class="add-note-btn" onClick={handleAddNote}>+ {t('list.add_note')}</button>
            </div>
          </Show>
        </div>

        {/* Back Side (Config) */}
        <div 
          class="list-card-back"
          onClick={(e) => e.stopPropagation()}
        >
          <div class="list-config-panel">
            <div class="list-config-header">
              <h4>{props.list.id.toString().startsWith('temp-') ? t('list.new_list') : t('list.configure')}</h4>
              <button class="config-close-btn" onClick={handleCancel}>×</button>
            </div>
            
            <div class="config-form-group">
              <label for={`list-title-${props.list.id}`}>{t('list.title_label')}</label>
              <input 
                id={`list-title-${props.list.id}`}
                ref={titleInputRef}
                type="text" 
                value={title()} 
                onInput={(e) => setTitle(e.currentTarget.value)} 
                onKeyDown={handleKeyDown}
                class="config-list-title"
                placeholder={t('list.title_placeholder')}
              />
            </div>

            <ColorPicker listId={props.list.id} value={color()} onChange={setColor} />

            <Show when={!props.list.id.toString().startsWith('temp-')}>
              <div class="config-form-group">
                <label for={`list-board-target-${props.list.id}`}>{t('list.move_copy_label')}</label>
                <select id={`list-board-target-${props.list.id}`} class="config-board-target" onChange={(e) => {
                  const targetId = parseInt(e.currentTarget.value);
                  setTargetBoardId(targetId);
                }}>
                  <option value="">{t('list.move_copy_placeholder')}</option>
                  <For each={boards.filter(b => b.id !== currentBoard.id)}>
                    {(board) => <option value={board.id}>{board.title}</option>}
                  </For>
                </select>
                <div class="config-board-actions">
                  <button 
                    class="config-copy-btn" 
                    disabled={!targetBoardId()}
                    onClick={() => copyOrMoveList(props.list.id, targetBoardId(), true)}
                  >
                    {t('list.copy')}
                  </button>
                  <button 
                    class="config-move-btn" 
                    disabled={!targetBoardId()}
                    onClick={() => copyOrMoveList(props.list.id, targetBoardId(), false)}
                  >
                    {t('list.move')}
                  </button>
                </div>
              </div>
            </Show>

            <div class="config-actions">
              <Show when={!props.list.id.toString().startsWith('temp-')}>
                <button class="config-delete-btn" onClick={() => deleteList(props.list.id)}>{t('list.delete')}</button>
              </Show>
              <button class="config-cancel-btn" onClick={handleCancel}>{t('item.cancel')}</button>
              <button class="config-save-btn" onClick={handleSave}>{t('item.save')}</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
