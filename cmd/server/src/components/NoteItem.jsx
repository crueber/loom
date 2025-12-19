import { Show, createSignal, createEffect } from 'solid-js';
import { ItemHeader } from './ItemHeader';

export function NoteItem(props) {
  const [isFlipped, setIsFlipped] = createSignal(props.item.id.toString().startsWith('temp-'));
  const [content, setContent] = createSignal(props.item.content || '');
  let textareaRef;

  createEffect(() => {
    if (isFlipped() && textareaRef) {
      textareaRef.focus();
      textareaRef.select();
    }
  });

  const handleSave = () => {
    const updates = { content: content() };
    if (props.item.id.toString().startsWith('temp-')) {
      props.onSaveNew(props.item.id, updates);
    } else {
      props.onUpdate(props.item.id, updates);
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
    if (props.item.id.toString().startsWith('temp-')) {
      props.onDelete();
    } else {
      setIsFlipped(false);
      setContent(props.item.content || '');
    }
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
              <ItemHeader title="Edit Note" onClose={handleCancel} />
              <label for={`note-content-${props.item.id}`} class="sr-only">Content</label>
              <textarea 
                id={`note-content-${props.item.id}`}
                ref={textareaRef}
                value={content()} 
                onInput={(e) => setContent(e.currentTarget.value)} 
                onKeyDown={handleKeyDown}
                placeholder="Note content (Markdown supported)"
                rows="5"
              />
              <div class="item-config-actions">
                <Show when={!props.item.id.toString().startsWith('temp-')}>
                  <button class="item-config-delete-btn" onClick={props.onDelete}>Delete</button>
                </Show>
                <button class="item-config-cancel-btn secondary" onClick={handleCancel}>Cancel</button>
                <button class="item-config-save-btn" onClick={handleSave}>Save</button>
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
