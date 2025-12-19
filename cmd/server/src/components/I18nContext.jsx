import { createContext, useContext } from 'solid-js';

const I18nContext = createContext();

export function I18nProvider(props) {
  const translations = window.__I18N_DATA__ || {};

  // Helper to get nested keys like 'app.name'
  const t = (path) => {
    const keys = path.split('.');
    let result = translations;
    for (const key of keys) {
      if (result && result[key] !== undefined) {
        result = result[key];
      } else {
        return path; // Fallback to key name if not found
      }
    }
    return result;
  };

  return (
    <I18nContext.Provider value={{ t }}>
      {props.children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}
