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
export function localTimeIn(timezone: string, now: Date = new Date()): string {
  return new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    hour: "numeric",
    minute: "2-digit",
  }).format(now);
}

/** Offset label relative to the viewer, e.g. "6 hours ahead of you". */
export function offsetFromViewer(timezone: string, now: Date = new Date()): string | null {
  const viewerTz =
    Intl.DateTimeFormat().resolvedOptions().timeZone || DEFAULT_VIEWER_TZ;
  if (viewerTz === timezone) return null;

  const there = zonedOffsetMinutes(timezone, now);
  const here = zonedOffsetMinutes(viewerTz, now);
  const diffHours = Math.round((there - here) / 60);

  if (diffHours === 0) return "same time as you";
  const magnitude = Math.abs(diffHours);
  const unit = magnitude === 1 ? "hour" : "hours";
  return `${magnitude} ${unit} ${diffHours > 0 ? "ahead of" : "behind"} you`;
}

/** Minutes east of UTC for a timezone at a given instant. */
function zonedOffsetMinutes(timeZone: string, now: Date): number {
  // Compare the same instant rendered in the target zone vs. UTC.
  const tzDate = new Date(now.toLocaleString("en-US", { timeZone }));
  const utcDate = new Date(now.toLocaleString("en-US", { timeZone: "UTC" }));
  return (tzDate.getTime() - utcDate.getTime()) / 60000;
}
