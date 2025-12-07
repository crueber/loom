// Shared Card Flip Behavior

// Global state for currently flipped card
let currentlyFlippedCard = null;

// Track Sortable instances
let listsSortable = null;
let bookmarkSortables = {};

function getCurrentlyFlippedCard() {
    return currentlyFlippedCard;
}

function setCurrentlyFlippedCard(card) {
    currentlyFlippedCard = card;
}

function closeFlippedCard() {
    if (!currentlyFlippedCard) return;

    if (currentlyFlippedCard.type === 'list') {
        const card = document.querySelector(`.list-card[data-list-id="${currentlyFlippedCard.id}"]`);
        if (card) {
            // Remove if temp
            if (card.dataset.isTemp === 'true') {
                // Remove from lists array - will be handled by component
                const event = new CustomEvent('removeTempList', { detail: { id: currentlyFlippedCard.id } });
                document.dispatchEvent(event);
            } else {
                card.dataset.flipped = 'false';
            }
        }
    } else if (currentlyFlippedCard.type === 'bookmark') {
        const bookmark = document.querySelector(`.bookmark-item[data-item-id="${currentlyFlippedCard.id}"]`);
        if (bookmark) {
            // Remove if temp
            if (bookmark.dataset.isTemp === 'true') {
                bookmark.remove();
            } else {
                bookmark.dataset.flipped = 'false';
            }
        }
    } else if (currentlyFlippedCard.type === 'note') {
        const note = document.querySelector(`.note-item[data-item-id="${currentlyFlippedCard.id}"]`);
        if (note) {
            // Remove if temp
            if (note.dataset.isTemp === 'true') {
                note.remove();
            } else {
                note.dataset.flipped = 'false';
            }
        }
    }

    currentlyFlippedCard = null;

    // Re-enable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", false);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", false);
    });
}

function flipToList(listId) {
    // Close currently flipped card if it's different
    if (currentlyFlippedCard && (currentlyFlippedCard.type !== 'list' || currentlyFlippedCard.id !== listId)) {
        closeFlippedCard();
    }

    const card = document.querySelector(`.list-card[data-list-id="${listId}"]`);
    if (!card) return;

    card.dataset.flipped = 'true';
    currentlyFlippedCard = { type: 'list', id: listId };

    // Disable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", true);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", true);
    });

    // Dispatch event for component to handle form reset
    const event = new CustomEvent('listFlipped', { detail: { listId } });
    document.dispatchEvent(event);
}

function flipToBookmark(bookmarkId) {
    // Close currently flipped card if it's different
    if (currentlyFlippedCard && (currentlyFlippedCard.type !== 'bookmark' || currentlyFlippedCard.id !== bookmarkId)) {
        closeFlippedCard();
    }

    const bookmarkEl = document.querySelector(`.bookmark-item[data-item-id="${bookmarkId}"]`);
    if (!bookmarkEl) return;

    bookmarkEl.dataset.flipped = 'true';
    currentlyFlippedCard = { type: 'bookmark', id: bookmarkId };

    // Disable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", true);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", true);
    });

    // Dispatch event for component to handle form reset
    const event = new CustomEvent('bookmarkFlipped', { detail: { bookmarkId } });
    document.dispatchEvent(event);
}

function flipToNote(noteId) {
    // Close currently flipped card if it's different
    if (currentlyFlippedCard && (currentlyFlippedCard.type !== 'note' || currentlyFlippedCard.id !== noteId)) {
        closeFlippedCard();
    }

    const noteEl = document.querySelector(`.note-item[data-item-id="${noteId}"]`);
    if (!noteEl) return;

    noteEl.dataset.flipped = 'true';
    currentlyFlippedCard = { type: 'note', id: noteId };

    const textarea = noteEl.querySelector(`textarea`);
    textarea.focus();
    textarea.selectionStart = textarea.value.length

    // Disable all Sortable instances
    if (listsSortable) {
        listsSortable.option("disabled", true);
    }
    Object.values(bookmarkSortables).forEach(sortable => {
        sortable.option("disabled", true);
    });

    // Dispatch event for component to handle form reset
    const event = new CustomEvent('noteFlipped', { detail: { noteId } });
    document.dispatchEvent(event);
}

// Keyboard event listener
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && currentlyFlippedCard) {
        closeFlippedCard();
    }
});
