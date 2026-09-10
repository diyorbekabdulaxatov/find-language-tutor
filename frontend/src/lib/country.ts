/** ISO 3166-1 alpha-2 -> flag emoji, e.g. "UZ" -> "🇺🇿". */
export function flagEmoji(countryCode: string): string {
  const code = countryCode.trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(code)) return "";
  const OFFSET = 0x1f1e6 - "A".charCodeAt(0);
  return String.fromCodePoint(
    ...[...code].map((c) => c.charCodeAt(0) + OFFSET),
  );
}
