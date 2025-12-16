// Loom - Main App Initialization
// Alpine.js component framework with modular architecture

// Import dependencies
import { exportData, importData, showError, hideError, dispatchEvent } from './utils/api.js';
import { initFlipCardStore, initFlipCardListeners } from './components/flipCard.js';
import { initializeHorizontalDragScroll } from './components/dragScroll.js';
import { bootstrapData } from './components/dataBootstrap.js';
import { Events } from './components/events.js';

// Initialize Alpine stores before components load
document.addEventListener('alpine:init', () => {
    initFlipCardStore();
});

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

// Bootstrap application data on Alpine initialized
document.addEventListener('alpine:initialized', async () => {
    await bootstrapData();
})


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

        window.location.reload();
    } catch (error) {
        console.error('Import failed:', error);
        showError('import-error', error.message);
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
