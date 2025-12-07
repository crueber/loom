// Horizontal Drag Scrolling Component

function initializeHorizontalDragScroll(container, currentlyFlippedCardGetter) {
    let isDown = false;
    let startX;
    let scrollLeft;

    container.addEventListener('mousedown', (e) => {
        // Don't activate if any card is flipped
        if (currentlyFlippedCardGetter()) {
            return;
        }

        // Don't activate on input elements, buttons, or other interactive elements
        // Check both target and closest parent to handle labels
        let element = e.target;
        while (element && element !== container) {
            if (element.tagName === 'INPUT' ||
                element.tagName === 'BUTTON' ||
                element.tagName === 'A' ||
                element.tagName === 'TEXTAREA' ||
                element.tagName === 'SELECT' ||
                element.tagName === 'LABEL') {
                return;
            }
            element = element.parentElement;
        }

        // Only activate on container background or bookmarks-container whitespace
        const isValidTarget = e.target === container ||
                             e.target.classList.contains('bookmarks-container') ||
                             e.target.classList.contains('lists-wrapper');

        if (!isValidTarget) return;

        isDown = true;
        container.style.cursor = 'grabbing';
        startX = e.pageX - container.offsetLeft;
        scrollLeft = container.scrollLeft;
        e.preventDefault(); // Prevent text selection when dragging valid targets
    });

    container.addEventListener('mouseleave', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mouseup', () => {
        isDown = false;
        container.style.cursor = 'grab';
    });

    container.addEventListener('mousemove', (e) => {
        if (!isDown) return;

        // Don't prevent default on interactive elements
        const target = e.target;
        if (target.tagName !== 'INPUT' &&
            target.tagName !== 'BUTTON' &&
            target.tagName !== 'A' &&
            target.tagName !== 'TEXTAREA' &&
            target.tagName !== 'SELECT') {
            e.preventDefault();
        }

        const x = e.pageX - container.offsetLeft;
        const walk = (x - startX) * 2;
        container.scrollLeft = scrollLeft - walk;
    });

    // Set initial cursor
    container.style.cursor = 'grab';
}
