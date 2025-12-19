import { createSignal, createContext, useContext } from 'solid-js';
import { createStore } from 'solid-js/store';
import { 
  updateBoard as apiUpdateBoard, 
  deleteBoard as apiDeleteBoard, 
  reorderItems as apiReorderItems, 
  reorderLists as apiReorderLists,
  createList as apiCreateList,
  deleteList as apiDeleteList,
  updateList as apiUpdateList,
  createItem as apiCreateItem,
  deleteItem as apiDeleteItem,
  updateItem as apiUpdateItem,
  copyOrMoveList as apiCopyOrMoveList
} from '../utils/api';

const BoardContext = createContext();

export function BoardProvider(props) {
  const bootstrapData = window.__BOOTSTRAP_DATA__ || {};
  
  const [boards, setBoards] = createStore(bootstrapData.boards || []);
  const [currentBoard, setCurrentBoard] = createStore(bootstrapData.board || { title: 'Loading...' });
  const [lists, setLists] = createStore(bootstrapData.lists || []);
  const [items, setItems] = createStore(bootstrapData.items || {});

  const createBoard = async () => {
    try {
      const response = await fetch('/api/boards', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title: 'New Board' })
      });

      if (!response.ok) {
        throw new Error('Failed to create board');
      }

      const newBoard = await response.json();
      window.location.href = `/boards/${newBoard.id}`;
    } catch (error) {
      console.error('Failed to create board:', error);
      alert('Failed to create board: ' + error.message);
    }
  };

  const updateBoard = async (id, title) => {
    try {
      await apiUpdateBoard(id, title);
      if (currentBoard.id === id) {
        setCurrentBoard('title', title);
      }
      const index = boards.findIndex(b => b.id === id);
      if (index !== -1) {
        setBoards(index, 'title', title);
      }
    } catch (error) {
      console.error('Failed to update board title:', error);
      alert('Failed to update board title: ' + error.message);
    }
  };

  const deleteBoard = async (id) => {
    if (boards.length <= 1) {
      alert('Cannot delete the only board');
      return;
    }

    try {
      await apiDeleteBoard(id);
      const remainingBoard = boards.find(b => b.id !== id);
      if (remainingBoard) {
        window.location.href = remainingBoard.is_default ? '/' : `/boards/${remainingBoard.id}`;
      } else {
        window.location.href = '/';
      }
    } catch (error) {
      console.error('Failed to delete board:', error);
      alert('Failed to delete board: ' + error.message);
    }
  };

  const addList = (title, color) => {
    const tempId = `temp-${Date.now()}`;
    const newList = {
      id: tempId,
      title,
      color,
      collapsed: false,
      board_id: currentBoard.id
    };
    setLists([...lists, newList]);
    setItems(tempId, []);
    return newList;
  };

  const saveNewList = async (tempId, data) => {
    try {
      const newList = await apiCreateList(data.title, data.color, currentBoard.id);
      setLists((prevLists) => 
        prevLists.map(list => list.id === tempId ? newList : list)
      );
      // Move items from temp list to new list
      if (items[tempId]) {
        setItems(newList.id, items[tempId]);
        setItems(tempId, undefined);
      }
      return newList;
    } catch (error) {
      console.error('Failed to save new list:', error);
      alert('Failed to save new list: ' + error.message);
      // Remove the temp list on failure
      setLists((prevLists) => prevLists.filter(list => list.id !== tempId));
    }
  };

  const deleteList = async (id) => {
    if (id.toString().startsWith('temp-')) {
      setLists(lists.filter(l => l.id !== id));
      setItems(id, undefined);
      return;
    }
    try {
      await apiDeleteList(id);
      setLists(lists.filter(l => l.id !== id));
      setItems(id, undefined);
    } catch (error) {
      console.error('Failed to delete list:', error);
      alert('Failed to delete list: ' + error.message);
    }
  };

  const addItem = (listId, type, data) => {
    const tempId = `temp-${Date.now()}`;
    const newItem = {
      id: tempId,
      list_id: listId,
      type,
      ...data,
      title: data.title || (type === 'bookmark' ? 'New Link' : ''),
      url: data.url || (type === 'bookmark' ? 'https://' : ''),
      content: data.content || ''
    };
    setItems(listId, [...(items[listId] || []), newItem]);
    return newItem;
  };

  const saveNewItem = async (tempId, listId, type, data) => {
    try {
      const newItem = await apiCreateItem(listId, type, data);
      setItems(listId, (prevItems) => 
        prevItems.map(item => item.id === tempId ? newItem : item)
      );
      return newItem;
    } catch (error) {
      console.error('Failed to save new item:', error);
      alert('Failed to save new item: ' + error.message);
      // Remove the temp item on failure
      setItems(listId, (prevItems) => prevItems.filter(item => item.id !== tempId));
    }
  };

  const deleteItem = async (id, listId) => {
    if (id.toString().startsWith('temp-')) {
      setItems(listId, items[listId].filter(i => i.id !== id));
      return;
    }
    try {
      await apiDeleteItem(id);
      setItems(listId, items[listId].filter(i => i.id !== id));
    } catch (error) {
      console.error('Failed to delete item:', error);
      alert('Failed to delete item: ' + error.message);
    }
  };

  const updateItem = async (id, listId, updates) => {
    try {
      const updatedItem = await apiUpdateItem(id, updates);
      const index = items[listId].findIndex(i => i.id === id);
      if (index !== -1) {
        setItems(listId, index, updatedItem);
      }
    } catch (error) {
      console.error('Failed to update item:', error);
      alert('Failed to update item: ' + error.message);
    }
  };

  const updateList = async (id, updates) => {
    try {
      const updatedList = await apiUpdateList(id, updates);
      const index = lists.findIndex(l => l.id === id);
      if (index !== -1) {
        setLists(index, updatedList);
      }
    } catch (error) {
      console.error('Failed to update list:', error);
      alert('Failed to update list: ' + error.message);
    }
  };

  const moveItem = async (itemId, fromListId, toListId, newPosition) => {
    const item = items[fromListId].find(i => i.id === itemId);
    if (!item) return;

    // Optimistic update
    const isSameList = fromListId === toListId;
    let newFromList = items[fromListId].filter(i => i.id !== itemId);
    let newToList = isSameList ? [...newFromList] : [...(items[toListId] || [])];
    
    const updatedItem = { ...item, list_id: toListId };
    newToList.splice(newPosition, 0, updatedItem);

    if (isSameList) {
      setItems(fromListId, newToList);
    } else {
      setItems(fromListId, newFromList);
      setItems(toListId, newToList);
    }

    try {
      const reorderData = [];
      
      // Add items from the source list if it's different
      if (!isSameList) {
        newFromList.forEach((it, idx) => {
          reorderData.push({
            id: it.id,
            position: idx,
            list_id: fromListId
          });
        });
      }

      // Add items from the target list
      newToList.forEach((it, idx) => {
        reorderData.push({
          id: it.id,
          position: idx,
          list_id: toListId
        });
      });

      await apiReorderItems(reorderData);
    } catch (error) {
      console.error('Failed to move item:', error);
    }
  };

  const reorderLists = async (fromIndex, toPosition) => {
    const newLists = [...lists];
    const [movedList] = newLists.splice(fromIndex, 1);
    newLists.splice(toPosition, 0, movedList);

    // Use a fresh array to ensure SolidJS detects the change
    setLists([...newLists]);

    try {
      const reorderData = newLists.map((l, idx) => ({
        id: l.id,
        position: idx
      }));
      await apiReorderLists(reorderData);
    } catch (error) {
      console.error('Failed to reorder lists:', error);
      window.location.reload();
    }
  };

  const copyOrMoveList = async (listId, targetBoardId, copy) => {
    try {
      await apiCopyOrMoveList(listId, targetBoardId, copy);
      // Refresh to show changes (simplest approach for cross-board moves)
      window.location.reload();
    } catch (error) {
      console.error('Failed to copy/move list:', error);
      alert('Failed to copy/move list: ' + error.message);
    }
  };

  const board = {
    boards,
    currentBoard,
    lists,
    items,
    createBoard,
    updateBoard,
    deleteBoard,
    addList,
    saveNewList,
    deleteList,
    updateList,
    addItem,
    saveNewItem,
    deleteItem,
    updateItem,
    moveItem,
    reorderLists,
    copyOrMoveList,
    setLists,
    setItems
  };

  return (
    <BoardContext.Provider value={board}>
      {props.children}
    </BoardContext.Provider>
  );
}

export function useBoard() {
  return useContext(BoardContext);
}
