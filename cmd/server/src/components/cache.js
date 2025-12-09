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

function hasDataChanged(oldData, newData) {
    // Simple deep comparison via JSON stringify
    // More efficient than comparing field by field for small datasets
    return JSON.stringify(oldData) !== JSON.stringify(newData);
}

export { saveToCache, loadFromCache, hasDataChanged };
