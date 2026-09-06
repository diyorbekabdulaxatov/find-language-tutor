import type { LanguageLevel, Money } from "@/types/teacher";

/**
 * Render a Money value for display:
 *   { 12000000, "UZS" } -> "120,000 so'm"
 *   { 1200, "USD" }     -> "$12"
 * UZS is shown with a plain "so'm" suffix (the ISO symbol "UZS" reads as a code,
 * not money) and never with fractional units.
 */
export function formatMoney(money: Money): string {
  const { amountMinor, currency } = money;

  if (currency === "UZS") {
    const soms = Math.round(amountMinor / 100);
    return `${new Intl.NumberFormat("en-US").format(soms)} so'm`;
  }

  const hasCents = amountMinor % 100 !== 0;
  return new Intl.NumberFormat("en-US", {
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

/** "2h" / "under an hour" / "1 day" — for response-time copy. */
export function formatResponseTime(hours: number): string {
  if (hours < 1) return "under an hour";
  if (hours < 24) return `${Math.round(hours)}h`;
  const days = Math.round(hours / 24);
  return days === 1 ? "1 day" : `${days} days`;
}

/** Compact counts for stats: 1200 -> "1.2k". */
export function formatCompact(n: number): string {
  return new Intl.NumberFormat("en-US", { notation: "compact" }).format(n);
}
