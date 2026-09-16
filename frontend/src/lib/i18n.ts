import type { useTranslations } from "next-intl";

/** The `languages` namespace translator (same shape from `getTranslations`). */
type LanguagesT = ReturnType<typeof useTranslations<"languages">>;

/**
 * A taught language's display name in the UI locale. The backend sends the
 * English name; the catalogue covers the codes we know and the English name
 * is the fallback for any it doesn't.
 */
export function languageName(t: LanguagesT, lang: { code: string; name: string }): string {
  return t.has(lang.code as never) ? t(lang.code as never) : lang.name;
}

/** A greeting in each language we currently have teachers for, used as a motif. */
const GREETINGS: Record<string, string> = {
  en: "Hello",
  ru: "Привет",
  uz: "Salom",
  tr: "Merhaba",
  ko: "안녕하세요",
  de: "Hallo",
  ar: "مرحبا",
};

/** Learners are in Uzbekistan; fall back to Tashkent when the browser tz is unknown. */
export const DEFAULT_VIEWER_TZ = "Asia/Tashkent";

export function greetingFor(languageCode: string): string {
  return GREETINGS[languageCode] ?? "Hello";
}

/**
 * The teacher's wall-clock time right now, formatted like "8:14 PM".
 * `Intl.DateTimeFormat` does the timezone math from the IANA name — no date
 * library needed. Pass a fixed `now` on the server so SSR output is stable;
 * the client component re-renders with the live value.
 */
export function localTimeIn(timezone: string, now: Date = new Date(), locale = "en"): string {
  return new Intl.DateTimeFormat(locale, {
    timeZone: timezone,
    hour: "numeric",
    minute: "2-digit",
  }).format(now);
}

/**
 * Whole hours the teacher's zone is ahead of (positive) or behind (negative)
 * the viewer's; null when they share a zone. The caller words it.
 */
export function hoursFromViewer(timezone: string, now: Date = new Date()): number | null {
  const viewerTz =
    Intl.DateTimeFormat().resolvedOptions().timeZone || DEFAULT_VIEWER_TZ;
  if (viewerTz === timezone) return null;

  const there = zonedOffsetMinutes(timezone, now);
  const here = zonedOffsetMinutes(viewerTz, now);
  return Math.round((there - here) / 60);
}

/** Minutes east of UTC for a timezone at a given instant. */
function zonedOffsetMinutes(timeZone: string, now: Date): number {
  // Compare the same instant rendered in the target zone vs. UTC.
  const tzDate = new Date(now.toLocaleString("en-US", { timeZone }));
  const utcDate = new Date(now.toLocaleString("en-US", { timeZone: "UTC" }));
  return (tzDate.getTime() - utcDate.getTime()) / 60000;
}
