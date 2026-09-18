import type { LanguageLevel, Money } from "@/types/teacher";

/** The so'm suffix per UI locale — the ISO code "UZS" reads as a code, not money. */
const SOM_SUFFIX: Record<string, string> = { en: "so'm", uz: "so'm", ru: "сум" };

/**
 * Render a Money value for display:
 *   { 12000000, "UZS" } -> "120,000 so'm"   (en)  /  "120 000 сум" (ru)
 *   { 1200, "USD" }     -> "$12"
 * UZS never shows fractional units. `locale` is the UI locale (from
 * `useLocale()` / `getLocale()`); it drives digit grouping and the suffix.
 */
export function formatMoney(money: Money, locale = "en"): string {
  const { amountMinor, currency } = money;

  if (currency === "UZS") {
    const soms = Math.round(amountMinor / 100);
    return `${new Intl.NumberFormat(locale).format(soms)} ${SOM_SUFFIX[locale] ?? "so'm"}`;
  }

  const hasCents = amountMinor % 100 !== 0;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: hasCents ? 2 : 0,
    maximumFractionDigits: hasCents ? 2 : 0,
  }).format(amountMinor / 100);
}

const LEVEL_LABEL: Record<LanguageLevel, string> = {
  native: "Native",
  c2: "C2 · Proficient",
  c1: "C1 · Advanced",
  b2: "B2 · Upper-intermediate",
  b1: "B1 · Intermediate",
  a2: "A2 · Elementary",
  a1: "A1 · Beginner",
};

export function formatLevel(level: LanguageLevel): string {
  return LEVEL_LABEL[level];
}

/** Compact counts for stats: 1200 -> "1.2k". */
export function formatCompact(n: number, locale = "en"): string {
  return new Intl.NumberFormat(locale, { notation: "compact" }).format(n);
}

/**
 * A lecture's own runtime, as a player shows it: "4:12", "1:02:30".
 * Locale-independent on purpose — a timecode is read as a timecode in every
 * language, and `Intl` has no format for one.
 *
 * Returns null for 0, which means "unknown duration" throughout the courses
 * module (a container the browser couldn't read a duration from, or an item
 * created before the field existed). Callers render nothing rather than
 * "0:00", so a missing figure never looks like an empty lecture.
 */
export function formatLectureLength(seconds: number): string | null {
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  const total = Math.round(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

/**
 * A whole course's length, as a catalog card shows it: "3h 20m", "45m".
 * Rounded to the minute — nobody buys a course by its seconds — and null for
 * 0 so the card omits the figure entirely.
 *
 * The unit labels are the caller's to supply (they are translated), which is
 * why this returns the parts rather than a string.
 */
export function courseLengthParts(seconds: number): { hours: number; minutes: number } | null {
  if (!Number.isFinite(seconds) || seconds <= 0) return null;
  const totalMinutes = Math.round(seconds / 60);
  // A course with some duration data must never render as "0m": round up to
  // one minute rather than disappearing.
  const safeMinutes = Math.max(totalMinutes, 1);
  return { hours: Math.floor(safeMinutes / 60), minutes: safeMinutes % 60 };
}
