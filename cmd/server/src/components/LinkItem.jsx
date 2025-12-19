import { Show, createSignal, createEffect } from 'solid-js';
import { ItemHeader } from './ItemHeader';
import { useI18n } from './I18nContext';

export function LinkItem(props) {
  const { t } = useI18n();
  const [isFlipped, setIsFlipped] = createSignal(props.item.id.toString().startsWith('temp-'));
  const [title, setTitle] = createSignal(props.item.title || '');
  const [url, setUrl] = createSignal(props.item.url || '');
  const [iconSource, setIconSource] = createSignal(props.item.icon_source || 'auto');
  const [customIconUrl, setCustomIconUrl] = createSignal(props.item.custom_icon_url || '');

  let titleInputRef;

  createEffect(() => {
    if (isFlipped() && props.item.id.toString().startsWith('temp-') && titleInputRef) {
      titleInputRef.focus();
      titleInputRef.select();
    }
  });

  const handleSave = () => {
    let finalUrl = url();
    if (finalUrl && !finalUrl.startsWith('http://') && !finalUrl.startsWith('https://')) {
      finalUrl = 'https://' + finalUrl;
    }

    const updates = { 
      title: title(), 
      url: finalUrl,
      icon_source: iconSource(),
      custom_icon_url: customIconUrl()
    };
    
    if (props.item.id.toString().startsWith('temp-')) {
      props.onSaveNew(props.item.id, updates);
    } else {
      props.onUpdate(props.item.id, updates);
      setIsFlipped(false);
      setUrl(finalUrl);
    }
  };

  const getDisplayUrl = (url) => {
    if (!url) return '';
    try {
      const urlObj = new URL(url);
      return urlObj.hostname;
    } catch (e) {
      // Fallback if URL is invalid
      return url;
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
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
              <ItemHeader title={t('item.edit_link')} onClose={handleCancel} />
              <div class="config-form-group">
                <label for={`link-title-${props.item.id}`} class="sr-only">{t('item.title_placeholder')}</label>
                <input 
                  id={`link-title-${props.item.id}`}
                  ref={titleInputRef}
                  type="text" 
                  value={title()} 
                  onInput={(e) => setTitle(e.currentTarget.value)} 
                  onKeyDown={handleKeyDown}
                  placeholder={t('item.title_placeholder')} 
                />
              </div>
              <div class="config-form-group">
                <label for={`link-url-${props.item.id}`} class="sr-only">{t('item.url_placeholder')}</label>
                <input 
                  id={`link-url-${props.item.id}`}
                  type="text" 
                  value={url()} 
                  onInput={(e) => setUrl(e.currentTarget.value)} 
                  onKeyDown={handleKeyDown}
                  placeholder={t('item.url_placeholder')} 
                />
              </div>
              
              <div class="favicon-config">
                <label for={`link-icon-source-${props.item.id}`} class="sr-only">{t('item.icon_source_label')}</label>
                <select id={`link-icon-source-${props.item.id}`} value={iconSource()} onChange={(e) => setIconSource(e.currentTarget.value)}>
                  <option value="auto">{t('item.icon_source_auto')}</option>
                  <option value="custom">{t('item.icon_source_custom')}</option>
                  <option value="service">{t('item.icon_source_service')}</option>
                </select>
                
                <Show when={iconSource() !== 'auto'}>
                  <div class="favicon-input-wrapper">
                    <label for={`link-custom-icon-${props.item.id}`} class="sr-only">
                      {iconSource() === 'custom' ? t('item.icon_source_custom') : t('item.icon_source_service')}
                    </label>
                    <input 
                      id={`link-custom-icon-${props.item.id}`}
                      type="text" 
                      value={customIconUrl()} 
                      onInput={(e) => setCustomIconUrl(e.currentTarget.value)} 
                      onKeyDown={handleKeyDown}
                      placeholder={iconSource() === 'custom' ? t('item.custom_icon_placeholder') : t('item.service_slug_placeholder')}
                    />
                    <Show when={iconSource() === 'service'}>
                      <a href="https://selfh.st/icons" target="_blank" rel="noopener noreferrer" class="icon-service-link">
                        {t('item.browse_icons')} ↗
                      </a>
                    </Show>
                  </div>
                </Show>
              </div>

              <div class="item-config-actions">
                <Show when={!props.item.id.toString().startsWith('temp-')}>
                  <button class="item-config-delete-btn" onClick={props.onDelete}>{t('item.delete')}</button>
                </Show>
                <button class="item-config-cancel-btn secondary" onClick={handleCancel}>{t('item.cancel')}</button>
                <button class="item-config-save-btn" onClick={handleSave}>{t('item.save')}</button>
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
              <p class="link-url">{getDisplayUrl(props.item.url)}</p>
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
