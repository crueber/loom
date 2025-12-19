import { Show, createSignal } from 'solid-js';
import { ItemHeader } from './ItemHeader';

export function LinkItem(props) {
  const [isFlipped, setIsFlipped] = createSignal(props.item.id.toString().startsWith('temp-'));
  const [title, setTitle] = createSignal(props.item.title || '');
  const [url, setUrl] = createSignal(props.item.url || '');
  const [iconSource, setIconSource] = createSignal(props.item.icon_source || 'auto');
  const [customIconUrl, setCustomIconUrl] = createSignal(props.item.custom_icon_url || '');

  const handleSave = () => {
    const updates = { 
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
      e.preventDefault();
      handleSave();
    }
  };

  const handleCancel = () => {
    if (props.item.id.toString().startsWith('temp-')) {
      props.onDelete();
    } else {
      setIsFlipped(false);
      setTitle(props.item.title || '');
      setUrl(props.item.url || '');
      setIconSource(props.item.icon_source || 'auto');
      setCustomIconUrl(props.item.custom_icon_url || '');
    }
  };

  return (
    <div class="link-item" data-id={props.item.id} data-flipped={isFlipped()}>
      <div class="item-card-inner">
        <Show when={!isFlipped()} fallback={
          <div class="item-card-back" onClick={(e) => e.stopPropagation()}>
            <div class="item-config-panel">
              <ItemHeader title="Edit Link" onClose={handleCancel} />
              <input 
                type="text" 
                value={title()} 
                onInput={(e) => setTitle(e.currentTarget.value)} 
                onKeyDown={handleKeyDown}
                placeholder="Title" 
              />
              <input 
                type="text" 
                value={url()} 
                onInput={(e) => setUrl(e.currentTarget.value)} 
                onKeyDown={handleKeyDown}
                placeholder="URL" 
              />
              
              <div class="favicon-config">
                <select value={iconSource()} onChange={(e) => setIconSource(e.currentTarget.value)}>
                  <option value="auto">Automatic Favicon</option>
                  <option value="custom">Custom URL</option>
                  <option value="service">selfh.st/icons</option>
                </select>
                
                <Show when={iconSource() !== 'auto'}>
                  <div class="favicon-input-wrapper">
                    <input 
                      type="text" 
                      value={customIconUrl()} 
                      onInput={(e) => setCustomIconUrl(e.currentTarget.value)} 
                      onKeyDown={handleKeyDown}
                      placeholder={iconSource() === 'custom' ? 'https://example.com/icon.png' : 'Service slug (e.g. plex)'}
                    />
                    <Show when={iconSource() === 'service'}>
                      <a href="https://selfh.st/icons" target="_blank" rel="noopener noreferrer" class="icon-service-link">
                        Browse Icons ↗
                      </a>
                    </Show>
                  </div>
                </Show>
              </div>

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
            class="link-card-front"
            onClick={() => {
              if (props.item.url) {
                window.location.href = props.item.url;
              }
            }}
            style={{ cursor: 'pointer' }}
          >
            <div class="link-favicon">
              <Show when={props.item.favicon_url} fallback={
                <div class="link-favicon-placeholder">🔗</div>
              }>
                <img src={props.item.favicon_url} alt="" />
              </Show>
            </div>
            <div class="link-content">
              <a 
                href={props.item.url} 
                class="link-title"
                onClick={(e) => e.stopPropagation()}
              >
                {props.item.title}
              </a>
              <p class="link-url">{props.item.url}</p>
            </div>
            <div class="link-actions">
              <button 
                class="item-action-btn gear-icon" 
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
