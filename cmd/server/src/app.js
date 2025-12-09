// Loom - Main App Initialization
// Alpine.js component framework with modular architecture

// Import dependencies
import { exportData, importData, showError, hideError } from './utils/api.js';
import { getCurrentlyFlippedCard, initFlipCardListeners } from './components/flipCard.js';
import { initializeHorizontalDragScroll } from './components/dragScroll.js';

// Import Alpine components (they register themselves)
import './components/auth.js';
import './components/boards.js';
import './components/lists.js';
import './components/items.js';

// Touch device detection
const isTouchDevice = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
if (isTouchDevice) {
    document.body.classList.add('touch-device');
}

// Export/Import Handlers
document.getElementById('export-btn').addEventListener('click', async () => {
    try {
        await exportData();
    } catch (error) {
        console.error('Export failed:', error);
        alert('Export failed');
    }
});

document.getElementById('import-btn').addEventListener('click', () => {
    document.getElementById('import-modal').showModal();
});

document.getElementById('close-import-modal').addEventListener('click', () => {
    document.getElementById('import-modal').close();
});

document.getElementById('cancel-import-btn').addEventListener('click', () => {
    document.getElementById('import-modal').close();
});

document.getElementById('confirm-import-btn').addEventListener('click', async () => {
    hideError('import-error');

    const fileInput = document.getElementById('import-file');
    const mode = document.getElementById('import-mode').value;

    if (!fileInput.files[0]) {
        showError('import-error', 'Please select a file');
        return;
    }

    try {
        const fileContent = await fileInput.files[0].text();
        const data = JSON.parse(fileContent);

        await importData(data, mode);
        document.getElementById('import-modal').close();
        document.getElementById('import-form').reset();

        // Reload data by dispatching event
        const event = new CustomEvent('reloadDataRequested');
        document.dispatchEvent(event);

        alert('Import successful!');
    } catch (error) {
        console.error('Import failed:', error);
        showError('import-error', error.message);
    }
});

// Color picker modal (currently unused, kept for compatibility)
document.getElementById('close-color-picker').addEventListener('click', () => {
    document.getElementById('color-picker-modal').close();
});

// Listen for reload data requests
document.addEventListener('reloadDataRequested', () => {
    // Trigger data reload in lists manager
    const listsManager = Alpine.$data(document.getElementById('lists-container'));
    if (listsManager && listsManager.loadData) {
        listsManager.loadData();
    }
});

// Initialize app on page load
(async () => {
    // Initialize flip card listeners
    initFlipCardListeners();

    // Initialize horizontal drag scrolling
    const container = document.getElementById('lists-container');
    if (container) {
        initializeHorizontalDragScroll(container);
    }
})();
