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
