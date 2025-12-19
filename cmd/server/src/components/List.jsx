import { For, Show, createSignal, onMount, createEffect } from 'solid-js';
import { Item } from './Item';
import { useBoard } from './BoardContext';
import { ColorPicker } from './ColorPicker';
import Sortable from 'sortablejs';

export function List(props) {
  const { items, updateItem, addItem, saveNewItem, deleteItem, deleteList, updateList, moveItem, reorderLists, lists, boards, currentBoard, copyOrMoveList } = useBoard();
  const [isFlipped, setIsFlipped] = createSignal(props.list.id.toString().startsWith('temp-'));
  const [isCollapsed, setIsCollapsed] = createSignal(props.list.collapsed);
  const [title, setTitle] = createSignal(props.list.title);
  const [color, setColor] = createSignal(props.list.color);
  const [targetBoardId, setTargetBoardId] = createSignal(0);
  const [itemsRef, setItemsRef] = createSignal();
  
  let titleInputRef;

  createEffect(() => {
    if (isFlipped() && props.list.id.toString().startsWith('temp-') && titleInputRef) {
      titleInputRef.focus();
    }
  });

  const listItems = () => items[props.list.id] || [];

  const handleUpdateItem = (itemId, updates) => {
    updateItem(itemId, props.list.id, updates);
  };

  const handleSaveNewItem = (tempId, updates) => {
    const item = listItems().find(i => i.id === tempId);
    if (item) {
      saveNewItem(tempId, props.list.id, item.type, updates);
    }
  };

  onMount(() => {
    const el = itemsRef();
    if (el && !props.list.id.toString().startsWith('temp-')) {
      Sortable.create(el, {
        group: 'items',
        animation: 150,
        ghostClass: 'sortable-ghost',
        chosenClass: 'sortable-chosen',
        dragClass: 'sortable-drag',
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
    }
  });

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSave();
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
            style={{ background: isCollapsed() ? 'none' : color() }} 
            onClick={handleToggleCollapse}
          >
            <h3 class="list-title">
              {props.list.title}
              <Show when={isCollapsed()}>
                {" — "}{listItems().length}
              </Show>
            </h3>
            <div class="list-actions" onClick={(e) => e.stopPropagation()}>
              <Show when={!isCollapsed()}>
                <button class="list-action-btn gear-icon" onClick={() => setIsFlipped(true)}>⚙️</button>
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
              <button class="add-bookmark-btn" onClick={handleAddLink}>+ Add Link</button>
              <button class="add-note-btn" onClick={handleAddNote}>+ Add Note</button>
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
              <h4>{props.list.id.toString().startsWith('temp-') ? 'New List' : 'Configure List'}</h4>
              <button class="config-close-btn" onClick={handleCancel}>×</button>
            </div>
            
            <div class="config-form-group">
              <label>Title</label>
              <input 
                ref={titleInputRef}
                type="text" 
                value={title()} 
                onInput={(e) => setTitle(e.currentTarget.value)} 
                onKeyDown={handleKeyDown}
                class="config-list-title"
                placeholder="List Title"
              />
            </div>

            <ColorPicker value={color()} onChange={setColor} />

            <Show when={!props.list.id.toString().startsWith('temp-')}>
              <div class="config-form-group">
                <label>Move/Copy to Board</label>
                <div class="config-board-actions">
                  <select class="config-board-target" onChange={(e) => {
                    const targetId = parseInt(e.currentTarget.value);
                    setTargetBoardId(targetId);
                  }}>
                    <option value="">Select target board...</option>
                    <For each={boards.filter(b => b.id !== currentBoard.id)}>
                      {(board) => <option value={board.id}>{board.title}</option>}
                    </For>
                  </select>
                  <button 
                    class="config-copy-btn" 
                    disabled={!targetBoardId()}
                    onClick={() => copyOrMoveList(props.list.id, targetBoardId(), true)}
                  >
                    Copy
                  </button>
                  <button 
                    class="config-move-btn" 
                    disabled={!targetBoardId()}
                    onClick={() => copyOrMoveList(props.list.id, targetBoardId(), false)}
                  >
                    Move
                  </button>
                </div>
              </div>
            </Show>

            <div class="config-actions">
              <Show when={!props.list.id.toString().startsWith('temp-')}>
                <button class="config-delete-btn" onClick={() => deleteList(props.list.id)}>Delete List</button>
              </Show>
              <button class="config-cancel-btn" onClick={handleCancel}>Cancel</button>
              <button class="config-save-btn" onClick={handleSave}>Save</button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
