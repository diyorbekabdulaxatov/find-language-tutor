/**
 * Weekly-availability calls. The GET is public (also used on the profile page
 * later); the PUT is owner-only and goes through the authed browser client.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { WeeklySlot } from "./timezone";

export interface WeeklyAvailability {
  teacherSlug: string;
  /** the teacher's IANA timezone */
  timezone: string;
  /** grid every slot boundary aligns to (currently 15 min) */
  granularityMinutes: number;
  /** UTC weekly slots, as stored */
  slots: WeeklySlot[];
}

export class AvailabilityError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "AvailabilityError";
    this.status = status;
  }
}

type ErrorBody = { error?: { message?: string } };

export async function getAvailability(
  slug: string,
): Promise<WeeklyAvailability> {
  const { data, error, response } = await browserApi.GET(
    "/v1/teachers/{slug}/availability",
    { params: { path: { slug } } },
  );
  if (error || !data) {
    throw new AvailabilityError(
      (error as ErrorBody | undefined)?.error?.message ??
        "Could not load your availability.",
      response.status,
    );
  }
  return {
    teacherSlug: data.teacher_slug,
    timezone: data.timezone,
    granularityMinutes: data.granularity_minutes,
    slots: data.slots.map((s) => ({
      weekday: s.weekday,
      startMinute: s.start_minute,
      endMinute: s.end_minute,
    })),
  };
}

/** Replace the whole weekly set. `utcSlots` must already be in UTC. */
export async function replaceAvailability(
  slug: string,
  utcSlots: WeeklySlot[],
): Promise<WeeklyAvailability> {
  const { data, error, response } = await browserApi.PUT(
    "/v1/teachers/{slug}/availability",
    {
      params: { path: { slug } },
      body: {
        slots: utcSlots.map((s) => ({
          weekday: s.weekday,
          start_minute: s.startMinute,
          end_minute: s.endMinute,
        })),
      },
    },
  );
  if (error || !data) {
    throw new AvailabilityError(
      (error as ErrorBody | undefined)?.error?.message ??
        "Could not save your availability.",
      response.status,
    );
  }
  return {
    teacherSlug: data.teacher_slug,
    timezone: data.timezone,
    granularityMinutes: data.granularity_minutes,
    slots: data.slots.map((s) => ({
      weekday: s.weekday,
      startMinute: s.start_minute,
      endMinute: s.end_minute,
    })),
  };
}
