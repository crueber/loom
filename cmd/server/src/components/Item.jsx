import { Show, createSignal, onMount, createEffect } from 'solid-js';

export function Item(props) {
  const [isFlipped, setIsFlipped] = createSignal(props.item.id.toString().startsWith('temp-'));
  const [title, setTitle] = createSignal(props.item.title || '');
  const [url, setUrl] = createSignal(props.item.url || '');
  const [content, setContent] = createSignal(props.item.content || '');

  const [iconSource, setIconSource] = createSignal(props.item.icon_source || 'auto');
  const [customIconUrl, setCustomIconUrl] = createSignal(props.item.custom_icon_url || '');

  let textareaRef;

  createEffect(() => {
    if (isFlipped() && props.item.type === 'note' && textareaRef) {
      textareaRef.focus();
      textareaRef.select();
    }
  });

  const handleSave = () => {
    const updates = props.item.type === 'note' 
      ? { content: content() }
      : { 
          title: title(), 
          url: url(),
          icon_source: iconSource(),
          custom_icon_url: customIconUrl()
        };
    
    if (props.item.id.toString().startsWith('temp-')) {
      props.onSaveNew(props.item.id, updates);
    } else {
      props.onUpdate(props.item.id, updates);
      setIsFlipped(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      if (props.item.type === 'note') {
        if (!e.shiftKey) {
          e.preventDefault();
          handleSave();
        }
      } else {
        // For bookmarks, any enter in an input should save
        e.preventDefault();
        handleSave();
      }
    }
  };

  const handleCancel = () => {
    if (props.item.id.toString().startsWith('temp-')) {
      props.onDelete();
    } else {
      setIsFlipped(false);
      // Reset local state to original values
      setTitle(props.item.title || '');
      setUrl(props.item.url || '');
      setContent(props.item.content || '');
      setIconSource(props.item.icon_source || 'auto');
      setCustomIconUrl(props.item.custom_icon_url || '');
    }
  };

  const renderMarkdown = (text) => {
    if (typeof window.marked === 'undefined') return text;
    return window.marked.parse(text);
  };

  return (
    <div 
      class={props.item.type === 'note' ? 'note-item' : 'bookmark-item'} 
      data-id={props.item.id} 
      data-flipped={isFlipped()}
    >
      <div class={props.item.type === 'note' ? 'note-card-inner' : 'bookmark-card-inner'}>
        <Show when={!isFlipped()} fallback={
          <div 
            class={props.item.type === 'note' ? 'note-card-back' : 'bookmark-card-back'}
            onClick={(e) => e.stopPropagation()}
          >
            <div class={props.item.type === 'note' ? 'note-config-panel' : 'bookmark-config-panel'}>
              <div class={props.item.type === 'note' ? 'bookmark-config-header' : 'bookmark-config-header'}>
                <h5>{props.item.type === 'note' ? 'Edit Note' : 'Edit Link'}</h5>
                <button class="bookmark-config-close-btn" onClick={handleCancel}>×</button>
              </div>
              <Show when={props.item.type === 'bookmark'}>
                <input 
                  type="text" 
                  value={title()} 
                  onInput={(e) => setTitle(e.currentTarget.value)} 
                  onKeyDown={handleKeyDown}
                  placeholder="Title" 
                />
              </Show>
              <Show when={props.item.type === 'bookmark'} fallback={
                <textarea 
                  ref={textareaRef}
                  value={content()} 
                  onInput={(e) => setContent(e.currentTarget.value)} 
                  onKeyDown={handleKeyDown}
                  placeholder="Note content (Markdown supported)"
                  rows="10"
                />
              }>
                <input 
                  type="text" 
                  value={url()} 
                  onInput={(e) => setUrl(e.currentTarget.value)} 
                  onKeyDown={handleKeyDown}
                  placeholder="URL" 
                />
                
                <div class="favicon-config">
                  <label>Favicon Source</label>
                  <select value={iconSource()} onChange={(e) => setIconSource(e.currentTarget.value)}>
                    <option value="auto">Automatic (from domain)</option>
                    <option value="custom">Custom URL</option>
                    <option value="service">Service (selfh.st/icons)</option>
                  </select>
                  
                  <Show when={iconSource() !== 'auto'}>
                    <input 
                      type="text" 
                      value={customIconUrl()} 
                      onInput={(e) => setCustomIconUrl(e.currentTarget.value)} 
                      onKeyDown={handleKeyDown}
                      placeholder={iconSource() === 'custom' ? 'https://example.com/icon.png' : 'Service slug (e.g. plex)'}
                    />
                  </Show>
                </div>
              </Show>
              <div class="bookmark-config-actions">
                <Show when={!props.item.id.toString().startsWith('temp-')}>
                  <button class="bookmark-config-delete-btn" onClick={props.onDelete}>Delete</button>
                </Show>
                <button class="bookmark-config-cancel-btn secondary" onClick={handleCancel}>Cancel</button>
                <button class="bookmark-config-save-btn" onClick={handleSave}>Save</button>
              </div>
            </div>
          </div>
        }>
          <div 
            class={props.item.type === 'note' ? 'note-card-front' : 'bookmark-card-front'}
            onClick={() => setIsFlipped(true)}
            style={{ cursor: 'pointer' }}
          >
            <Show when={props.item.type === 'bookmark'}>
              <div class="bookmark-favicon">
                <Show when={props.item.favicon_url} fallback={
                  <div class="bookmark-favicon-placeholder">🔗</div>
                }>
                  <img src={props.item.favicon_url} alt="" />
                </Show>
              </div>
            </Show>
            <div class={props.item.type === 'note' ? 'note-content' : 'bookmark-content'}>
              <Show when={props.item.type === 'bookmark'} fallback={
                <div class="note-text" innerHTML={renderMarkdown(content())} />
              }>
                <a 
                  href={props.item.url} 
                  target="_blank" 
                  rel="noopener noreferrer" 
                  class="bookmark-title"
                  onClick={(e) => e.stopPropagation()}
                >
                  {props.item.title}
                </a>
                <p class="bookmark-url">{props.item.url}</p>
              </Show>
            </div>
            <div class={props.item.type === 'note' ? 'note-actions' : 'bookmark-actions'}>
              <button 
                class="bookmark-action-btn gear-icon" 
                onClick={(e) => {
                  e.stopPropagation();
                  setIsFlipped(true);
                }}
              >
                ⚙️
              </button>
            </div>
          </div>
        </Show>
      </div>
    </div>
  );
}
