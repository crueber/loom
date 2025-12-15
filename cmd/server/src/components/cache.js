// LocalStorage Cache Management

const CACHE_KEY = 'loom-data-cache';

function saveToCache(data) {
    try {
        localStorage.setItem(CACHE_KEY, JSON.stringify(data));
    } catch (e) {
        console.warn('Failed to cache data:', e);
    }
}

function loadFromCache() {
    try {
        const cached = localStorage.getItem(CACHE_KEY);
        return cached ? JSON.parse(cached) : null;
    } catch (e) {
        console.warn('Failed to load cache:', e);
        return null;
    }
}

function updateCache(updates) {
    // Load current cache
    const cachedData = loadFromCache();
    if (cachedData) {
        // Update the cache with new data
        const updatedCache = {
            ...cachedData,
            ...updates
        };
        saveToCache(updatedCache);
    }
}

function hasDataChanged(oldData, newData) {
    // Fast shallow comparison to avoid expensive JSON.stringify
    // This is orders of magnitude faster for large datasets

    if (!oldData || !newData) {
        return true;
    }

    // Compare boards array length
    if (oldData.boards?.length !== newData.boards?.length) {
        return true;
    }

    // Compare lists array length
    if (oldData.lists?.length !== newData.lists?.length) {
        return true;
    }

    // Compare items structure (check each list's item count)
    const oldItems = oldData.items || {};
    const newItems = newData.items || {};
    const oldListIds = Object.keys(oldItems);
    const newListIds = Object.keys(newItems);

    if (oldListIds.length !== newListIds.length) {
        return true;
    }

    // Check item counts per list
    for (const listId of newListIds) {
        if (!oldItems[listId] || oldItems[listId].length !== newItems[listId].length) {
            return true;
        }
    }

    // If counts match, do a quick ID-based check on boards and lists
    // This catches renames, reorders, etc. without full serialization
    const boardsChanged = oldData.boards.some((board, i) =>
        board.id !== newData.boards[i]?.id ||
        board.title !== newData.boards[i]?.title
    );
    if (boardsChanged) {
        return true;
    }

    const listsChanged = oldData.lists.some((list, i) =>
        list.id !== newData.lists[i]?.id ||
        list.title !== newData.lists[i]?.title ||
        list.position !== newData.lists[i]?.position
    );
    if (listsChanged) {
        return true;
    }

    // If we get here, data appears unchanged
    return false;
}

export { saveToCache, loadFromCache, updateCache, hasDataChanged };
