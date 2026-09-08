/**
 * The backend stores weekly availability as UTC minutes-from-midnight on a
 * weekday (0 = Sunday). Teachers think in their own wall-clock time, so the
 * dashboard editor works in the teacher's IANA timezone and converts on the
 * boundary.
 *
 * A local weekly slot can straddle UTC midnight (e.g. Asia/Tashkent, UTC+5:
 * "Mon 02:00–07:00" local is "Sun 21:00 → Mon 02:00" UTC), so conversion may
 * split one slot into two — never more, since every offset is under 24h.
 *
 * Caveat: recurring weekly times are inherently ambiguous across DST for
 * abroad teachers. This uses the offset of an upcoming reference week; a
 * proper per-occurrence model is a later (booking-phase) concern.
 */

export interface WeeklySlot {
  /** 0 = Sunday … 6 = Saturday */
  weekday: number;
  /** minutes from 00:00 */
  startMinute: number;
  /** exclusive, minutes from 00:00; up to 1440 */
  endMinute: number;
}

const DAY = 1440;
const MS_PER_MIN = 60_000;

/** Minutes to add to a UTC instant to get wall-clock time in `tz`. */
function offsetMinutes(instant: Date, tz: string): number {
  const dtf = new Intl.DateTimeFormat("en-US", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
  const p = Object.fromEntries(
    dtf.formatToParts(instant).map((x) => [x.type, x.value]),
  );
  const asUTC = Date.UTC(
    Number(p.year),
    Number(p.month) - 1,
    Number(p.day),
    Number(p.hour === "24" ? "0" : p.hour),
    Number(p.minute),
    Number(p.second),
  );
  return Math.round((asUTC - instant.getTime()) / MS_PER_MIN);
}

/** The Sunday (UTC midnight) of the week that starts 7 days from now. */
function referenceSunday(): Date {
  const now = new Date();
  const sunday = Date.UTC(
    now.getUTCFullYear(),
    now.getUTCMonth(),
    now.getUTCDate() - now.getUTCDay() + 7,
  );
  return new Date(sunday);
}

/** Wall-clock time in `tz` → the UTC instant, for the reference week. */
function zonedToInstant(
  weekday: number,
  minute: number,
  tz: string,
  refSunday: Date,
): Date {
  const naive = refSunday.getTime() + (weekday * DAY + minute) * MS_PER_MIN;
  // Two passes settle all offsets except within a DST transition hour.
  let guess = new Date(naive);
  for (let i = 0; i < 2; i++) {
    const off = offsetMinutes(guess, tz);
    guess = new Date(naive - off * MS_PER_MIN);
  }
  return guess;
}

/** UTC instant → { weekday, minute } within the reference week. */
function instantToUtcParts(
  instant: Date,
  refSunday: Date,
): { weekday: number; minute: number } {
  const diffMin = Math.round(
    (instant.getTime() - refSunday.getTime()) / MS_PER_MIN,
  );
  const wrapped = ((diffMin % (7 * DAY)) + 7 * DAY) % (7 * DAY);
  return { weekday: Math.floor(wrapped / DAY), minute: wrapped % DAY };
}

/** Local weekly slots (in `tz`) → UTC weekly slots, splitting across midnight. */
export function localToUtc(slots: WeeklySlot[], tz: string): WeeklySlot[] {
  const ref = referenceSunday();
  const out: WeeklySlot[] = [];

  for (const s of slots) {
    const start = zonedToInstant(s.weekday, s.startMinute, tz, ref);
    const end = zonedToInstant(s.weekday, s.endMinute, tz, ref);
    const a = instantToUtcParts(start, ref);
    const b = instantToUtcParts(end, ref);

    const endMinute = b.minute === 0 && b.weekday !== a.weekday ? DAY : b.minute;

    if (a.weekday === b.weekday || endMinute === DAY) {
      out.push({ weekday: a.weekday, startMinute: a.minute, endMinute });
    } else {
      out.push({ weekday: a.weekday, startMinute: a.minute, endMinute: DAY });
      out.push({ weekday: b.weekday, startMinute: 0, endMinute: b.minute });
    }
  }
  return mergeAdjacent(out);
}

/** UTC weekly slots → local weekly slots (in `tz`), splitting across midnight. */
export function utcToLocal(slots: WeeklySlot[], tz: string): WeeklySlot[] {
  const ref = referenceSunday();
  const out: WeeklySlot[] = [];

  for (const s of slots) {
    const startNaive =
      ref.getTime() + (s.weekday * DAY + s.startMinute) * MS_PER_MIN;
    const endNaive =
      ref.getTime() + (s.weekday * DAY + s.endMinute) * MS_PER_MIN;

    const startOff = offsetMinutes(new Date(startNaive), tz);
    const endOff = offsetMinutes(new Date(endNaive), tz);

    const a = instantToUtcParts(new Date(startNaive + startOff * MS_PER_MIN), ref);
    const b = instantToUtcParts(new Date(endNaive + endOff * MS_PER_MIN), ref);

    const endMinute = b.minute === 0 && b.weekday !== a.weekday ? DAY : b.minute;

    if (a.weekday === b.weekday || endMinute === DAY) {
      out.push({ weekday: a.weekday, startMinute: a.minute, endMinute });
    } else {
      out.push({ weekday: a.weekday, startMinute: a.minute, endMinute: DAY });
      out.push({ weekday: b.weekday, startMinute: 0, endMinute: b.minute });
    }
  }
  return mergeAdjacent(out);
}

/** Merge touching slots on the same weekday (…endMinute === startMinute…). */
function mergeAdjacent(slots: WeeklySlot[]): WeeklySlot[] {
  const sorted = [...slots].sort(
    (x, y) => x.weekday - y.weekday || x.startMinute - y.startMinute,
  );
  const out: WeeklySlot[] = [];
  for (const s of sorted) {
    const last = out[out.length - 1];
    if (last && last.weekday === s.weekday && s.startMinute <= last.endMinute) {
      last.endMinute = Math.max(last.endMinute, s.endMinute);
    } else {
      out.push({ ...s });
    }
  }
  return out;
}

export function minutesToLabel(m: number): string {
  const h = Math.floor(m / 60);
  const min = m % 60;
  return `${String(h).padStart(2, "0")}:${String(min).padStart(2, "0")}`;
}
