import { For, createSignal, onMount } from 'solid-js';
import { List } from './List';
import { useBoard } from './BoardContext';
import { useDragScroll } from '../utils/useDragScroll';
import Sortable from 'sortablejs';

export function ListsManager() {
  const { lists, addList, saveNewList, deleteList, updateList, reorderLists } = useBoard();
  const [containerRef, setContainerRef] = createSignal();

  useDragScroll(containerRef);

  onMount(() => {
    const el = containerRef();
    if (el) {
      Sortable.create(el, {
        animation: 150,
        handle: '.list-header',
        draggable: '.list-card',
        ghostClass: 'sortable-ghost',
        chosenClass: 'sortable-chosen',
        dragClass: 'sortable-drag',
        forceFallback: true,
        fallbackClass: 'sortable-fallback',
        filter: '[data-flipped="true"], input, textarea, select, button',
        preventOnFilter: false,
        onEnd: (evt) => {
          if (evt.oldIndex !== evt.newIndex) {
            // Revert Sortable's DOM change so SolidJS can handle it
            const parent = evt.from;
            const item = evt.item;
            
            // Remove the item from its new position
            item.remove();
            
            // Re-insert it at its original position
            if (evt.oldIndex >= parent.children.length) {
              parent.appendChild(item);
            } else {
              parent.insertBefore(item, parent.children[evt.oldIndex]);
            }
            
            reorderLists(evt.oldIndex, evt.newIndex);
          }
        },
      });
    }
  });

  const handleAddList = () => {
    addList('', '#3D6D95');
  };

  return (
    <div id="lists-container" class="lists-wrapper" ref={setContainerRef}>
      <For each={lists}>
        {(list) => (
          <List 
            list={list} 
            onUpdate={updateList} 
            onSaveNew={saveNewList}
            onDelete={() => deleteList(list.id)}
          />
        )}
      </For>
      <div id="add-list-container">
        <button class="add-list-button" onClick={handleAddList}>+ Add List</button>
      </div>
    </div>
  );
}
