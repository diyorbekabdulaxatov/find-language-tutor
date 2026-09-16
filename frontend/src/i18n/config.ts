/**
 * Locale setup. The UI locale is a cookie, not a URL segment: most of the app
 * is behind a login where locale-prefixed URLs buy nothing, and keeping the
 * route tree flat avoids touching every `href` in the codebase. Prefixed URLs
 * for the public catalog can be layered on later for SEO without undoing this.
 */

export const LOCALES = ["en", "ru", "uz"] as const;
export type Locale = (typeof LOCALES)[number];

export const DEFAULT_LOCALE: Locale = "en";

export const LOCALE_COOKIE = "NEXT_LOCALE";

/** Native-language labels for the switcher (never translated). */
export const LOCALE_LABELS: Record<Locale, string> = {
  en: "English",
  ru: "Русский",
  uz: "O'zbekcha",
};

export function isLocale(value: string | undefined | null): value is Locale {
  return LOCALES.includes(value as Locale);
}

/**
 * Pick the best locale from an Accept-Language header — first listed
 * language whose primary subtag we support, else the default. Minimal on
 * purpose: `uz-Latn-UZ`, `ru-RU`, `en-GB` all resolve on the primary tag.
 */
export function negotiateLocale(acceptLanguage: string | null): Locale {
  if (!acceptLanguage) return DEFAULT_LOCALE;
  for (const part of acceptLanguage.split(",")) {
    const tag = part.split(";")[0]?.trim().toLowerCase();
    const primary = tag?.split("-")[0];
    if (isLocale(primary)) return primary;
  }
  return DEFAULT_LOCALE;
}
