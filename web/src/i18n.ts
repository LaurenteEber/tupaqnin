import en from "./i18n/en.json";
import es from "./i18n/es.json";

// Every on-screen string comes from the en/es catalogs (T-1). The
// demo runs in English (H-1); ?lang=es or the header switch picks
// Spanish. Keys missing in Spanish fall back to English.
export type Lang = "en" | "es";

const catalogs: Record<Lang, Record<string, string>> = {
  en: en as Record<string, string>,
  es: es as Record<string, string>,
};

export function initialLang(): Lang {
  const q = new URLSearchParams(window.location.search).get("lang");
  return q === "es" ? "es" : "en";
}

export function translate(lang: Lang, key: string, params?: Record<string, string>): string {
  let text = catalogs[lang][key] ?? catalogs.en[key] ?? key;
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      text = text.replace(`{${name}}`, value);
    }
  }
  return text;
}

// Model display names (U-2) are presentation: the API sends the model
// id; an unknown id shows as it came.
const modelNames: Record<string, string> = {
  "muse-spark-1.3": "Muse Spark 1.3",
  "muse-spark-1.2": "Muse Spark 1.2",
  "muse-spark-1.1": "Muse Spark 1.1",
  "muse-wonder-1.0": "Muse Wonder 1.0",
};

export function modelName(id: string): string {
  return modelNames[id] ?? id;
}

// The locale for numbers on screen (U-2: «0,24» in Spanish).
export function numberLocale(lang: Lang): string {
  return lang === "es" ? "es-ES" : "en-US";
}
