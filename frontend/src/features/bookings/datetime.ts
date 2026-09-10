/**
 * Display helpers for bookable slots. The backend hands us RFC3339 UTC
 * instants; `Intl.DateTimeFormat` with an IANA `timeZone` does all the
 * conversion, so no date library is needed.
 */

import type { BookableSlot } from "./api";

export function viewerTimezone(): string {
  return (
    Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Tashkent"
  );
}

/** "Mon 3 Feb" in the given zone. */
export function formatDayLabel(iso: string, tz: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    weekday: "short",
    day: "numeric",
    month: "short",
  }).format(new Date(iso));
}

/** "14:30" in the given zone. */
export function formatTime(iso: string, tz: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(iso));
}

/** "Monday, 3 February 2025 at 14:30" in the given zone. */
export function formatFull(iso: string, tz: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: tz,
    weekday: "long",
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(new Date(iso));
}

/** A stable YYYY-MM-DD key for the slot's date in a given zone. */
function dayKey(iso: string, tz: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(iso));
  return parts; // en-CA gives "YYYY-MM-DD"
}

export interface SlotDay {
  key: string;
  label: string;
  slots: BookableSlot[];
}

/** Group slots into days by the viewer's local calendar date. */
export function groupByDay(slots: BookableSlot[], viewerTz: string): SlotDay[] {
  const days = new Map<string, SlotDay>();
  for (const slot of slots) {
    const key = dayKey(slot.startAt, viewerTz);
    let day = days.get(key);
    if (!day) {
      day = { key, label: formatDayLabel(slot.startAt, viewerTz), slots: [] };
      days.set(key, day);
    }
    day.slots.push(slot);
  }
  return [...days.values()].sort((a, b) => a.key.localeCompare(b.key));
}
