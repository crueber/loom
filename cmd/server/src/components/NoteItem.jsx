import { Show, createSignal, createEffect } from 'solid-js';
import { ItemHeader } from './ItemHeader';
import { useI18n } from './I18nContext';

export function NoteItem(props) {
  const { t } = useI18n();
  const isTemp = () => props.item.id.toString().startsWith('temp-');
  const [isFlipped, setIsFlipped] = createSignal(isTemp());
  const [content, setContent] = createSignal(props.item.content || '');
  let textareaRef;

  const adjustHeight = () => {
    if (textareaRef) {
      textareaRef.style.height = 'auto';
      textareaRef.style.height = textareaRef.scrollHeight + 'px';
    }
  };

  createEffect(() => {
    if (isFlipped() && textareaRef) {
      textareaRef.focus();
      textareaRef.select();
      adjustHeight();
    }
  });

  // Keep the editor in sync with the persisted item while viewing, so a
  // reload or an external update is reflected without touching the draft
  // the user is actively editing.
  createEffect(() => {
    if (!isFlipped()) {
      setContent(props.item.content || '');
    }
  });

  const handleSave = () => {
    if (!isFlipped()) return;
    const next = content();
    const prev = props.item.content || '';
    if (isTemp()) {
      // Blurring/Enter on an untouched new note discards the draft instead
      // of persisting an empty card.
      if (!next.trim()) {
        props.onDelete();
        return;
      }
      props.onSaveNew(props.item.id, { content: next });
    } else {
      // Slice 2 empty-note semantic: editing an EXISTING note down to
      // empty/whitespace and blurring REVERTS to the persisted content
      // (no PUT, editor closes). Rationale: revert is non-destructive —
      // discarding (delete) on edit-to-empty would surprise-delete a note
      // the user may have cleared by accident, while persisting an empty
      // card would leave silent empty notes. New-note drafts keep the
      // Slice 1 semantic (empty draft discards on blur); explicit delete
      // stays on the Delete button.
      if (!next.trim()) {
        setContent(prev);
        setIsFlipped(false);
        return;
      }
      // No-op when nothing changed: close the editor without a PUT so a
      // stray blur never costs a write or a focus jump.
      if (next === prev) {
        setIsFlipped(false);
        return;
      }
      props.onUpdate(props.item.id, { content: next });
      setIsFlipped(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSave();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      handleCancel();
    }
  };

  const handleCancel = () => {
    if (isTemp()) {
      props.onDelete();
    } else {
      setIsFlipped(false);
      setContent(props.item.content || '');
    }
  };

  // Blur commits the draft in place: no modal, no navigation, no focus
  // jump. Save/Cancel buttons suppress the blur (mousedown default
  // prevented) so their explicit click handler owns the outcome and the
  // commit below never double-fires.
  const handleBlur = () => {
    handleSave();
  };

  const keepFocus = (e) => {
    e.preventDefault();
  };

  const renderMarkdown = (text) => {
    if (typeof window.marked === 'undefined') return text;
    return window.marked.parse(text);
  };

  return (
    <div class="note-item" data-id={props.item.id} data-flipped={isFlipped()}>
      <div class="item-card-inner">
        <Show when={!isFlipped()} fallback={
          <div class="item-card-back" onClick={(e) => e.stopPropagation()}>
            <div class="item-config-panel">
              <label for={`note-content-${props.item.id}`} class="sr-only">{t('item.note_placeholder')}</label>
              <textarea 
                id={`note-content-${props.item.id}`}
                ref={textareaRef}
                value={content()} 
                onInput={(e) => {
                  setContent(e.currentTarget.value);
                  adjustHeight();
                }} 
                onKeyDown={handleKeyDown}
                onBlur={handleBlur}
                placeholder={t('item.note_placeholder')}
                rows="1"
              />
              <div class="item-config-actions">
                <Show when={!isTemp()}>
                  <button class="item-config-delete-btn" onMouseDown={keepFocus} onClick={props.onDelete}>{t('item.delete')}</button>
                </Show>
                <button class="item-config-cancel-btn secondary" onMouseDown={keepFocus} onClick={handleCancel}>{t('item.cancel')}</button>
                <button class="item-config-save-btn" onMouseDown={keepFocus} onClick={handleSave}>{t('item.save')}</button>
              </div>
            </div>
          </div>
        }>
          <div 
            class="note-card-front"
            onClick={() => setIsFlipped(true)}
            style={{ cursor: 'pointer' }}
          >
            <div class="note-content">
              <div class="note-text" innerHTML={renderMarkdown(content())} />
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
