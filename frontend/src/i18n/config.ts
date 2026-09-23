import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import en from './locales/en/translation.json';
import de from './locales/de/translation.json';

export const SUPPORTED_LANGUAGES = ['en', 'de'] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

// Detects the browser language on first load, then remembers whatever the
// user picks via the language switcher (localStorage key `i18nextLng`).
void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      de: { translation: de },
    },
    fallbackLng: 'en',
    supportedLngs: SUPPORTED_LANGUAGES,
    nonExplicitSupportedLngs: true,
    interpolation: {
      escapeValue: false, // React already escapes.
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
    },
  });

// Keeps <html lang> in step so screen readers and hyphenation use the right language.
i18n.on('languageChanged', (lng) => {
  document.documentElement.lang = lng;
});
if (i18n.language) document.documentElement.lang = i18n.language;

export default i18n;
