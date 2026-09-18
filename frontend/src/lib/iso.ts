/**
 * ISO code lists for the profile pickers. Names come from `Intl.DisplayNames`
 * at runtime — in the viewer's locale for display, in English for what the
 * profile stores (`country_name` / language `name` are plain strings the
 * backend echoes back; the frontend re-localises known codes on read).
 */

/** Languages a teacher can list, most likely first. ISO 639-1. */
export const LANGUAGE_CODES = [
  "en", "ru", "uz", "kk", "ky", "tg", "tk", "tr", "ar", "fa", "zh", "ja", "ko",
  "de", "fr", "es", "it", "pt", "hi", "ur", "id", "ms", "vi", "th", "pl", "uk",
  "nl", "sv", "no", "da", "fi", "cs", "sk", "hu", "ro", "bg", "el", "he", "az",
  "hy", "ka", "mn", "sr", "hr", "sl", "lt", "lv", "et", "sw", "bn", "ta", "te",
  "ml", "pa", "ne", "si", "my", "km", "lo", "tl", "ps", "ku", "la",
];

/** ISO 3166-1 alpha-2, every assigned code. Uzbekistan and its neighbours first. */
export const COUNTRY_CODES = [
  "UZ", "KZ", "KG", "TJ", "TM", "RU", "TR", "KR", "CN", "US", "GB", "DE",
  "AD", "AE", "AF", "AG", "AI", "AL", "AM", "AO", "AQ", "AR", "AS", "AT", "AU", "AW", "AX", "AZ",
  "BA", "BB", "BD", "BE", "BF", "BG", "BH", "BI", "BJ", "BL", "BM", "BN", "BO", "BQ", "BR", "BS", "BT", "BV", "BW", "BY", "BZ",
  "CA", "CC", "CD", "CF", "CG", "CH", "CI", "CK", "CL", "CM", "CO", "CR", "CU", "CV", "CW", "CX", "CY", "CZ",
  "DJ", "DK", "DM", "DO", "DZ", "EC", "EE", "EG", "EH", "ER", "ES", "ET", "FI", "FJ", "FK", "FM", "FO", "FR",
  "GA", "GD", "GE", "GF", "GG", "GH", "GI", "GL", "GM", "GN", "GP", "GQ", "GR", "GS", "GT", "GU", "GW", "GY",
  "HK", "HM", "HN", "HR", "HT", "HU", "ID", "IE", "IL", "IM", "IN", "IO", "IQ", "IR", "IS", "IT",
  "JE", "JM", "JO", "JP", "KE", "KH", "KI", "KM", "KN", "KP", "KW", "KY",
  "LA", "LB", "LC", "LI", "LK", "LR", "LS", "LT", "LU", "LV", "LY",
  "MA", "MC", "MD", "ME", "MF", "MG", "MH", "MK", "ML", "MM", "MN", "MO", "MP", "MQ", "MR", "MS", "MT", "MU", "MV", "MW", "MX", "MY", "MZ",
  "NA", "NC", "NE", "NF", "NG", "NI", "NL", "NO", "NP", "NR", "NU", "NZ", "OM",
  "PA", "PE", "PF", "PG", "PH", "PK", "PL", "PM", "PN", "PR", "PS", "PT", "PW", "PY", "QA", "RE", "RO", "RS", "RW",
  "SA", "SB", "SC", "SD", "SE", "SG", "SH", "SI", "SJ", "SK", "SL", "SM", "SN", "SO", "SR", "SS", "ST", "SV", "SX", "SY", "SZ",
  "TC", "TD", "TF", "TG", "TH", "TL", "TN", "TO", "TT", "TV", "TW", "TZ", "UA", "UG", "UM", "UY",
  "VA", "VC", "VE", "VG", "VI", "VN", "VU", "WF", "WS", "YE", "YT", "ZA", "ZM", "ZW",
];

function displayNames(locale: string, type: "language" | "region"): Intl.DisplayNames | null {
  try {
    return new Intl.DisplayNames([locale], { type, fallback: "code" });
  } catch {
    return null;
  }
}

/** Human name for a code in `locale`, falling back to the code itself. */
export function regionName(code: string, locale: string): string {
  return displayNames(locale, "region")?.of(code) ?? code;
}

export function isoLanguageName(code: string, locale: string): string {
  return displayNames(locale, "language")?.of(code) ?? code;
}
