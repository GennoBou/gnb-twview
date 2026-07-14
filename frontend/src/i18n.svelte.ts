import ja from "./locales/ja.json";
import en from "./locales/en.json";

export const translations: Record<string, Record<string, string>> = { ja, en };

let currentLang = $state("en");

export const i18n = {
  get lang() {
    return currentLang;
  },
  set lang(value: string) {
    currentLang = value in translations ? value : "en";
  },
  t(key: string): string {
    const dict = translations[currentLang] || translations.en;
    return dict[key] || translations.en[key] || key;
  }
};
