/**
 * Authenticated teacher-profile calls for the dashboard. The public read layer
 * is `@/features/teachers/api` (server components, no credentials); this one
 * goes through the browser client so the owner's access token is attached.
 */

import { browserApi } from "@/features/auth/browser-client";
import { toMoney, toProfile, type LessonType } from "@/features/teachers/api";
import type { components } from "@/lib/api/schema";
import type { TeacherProfile } from "@/types/teacher";

type Writable = components["schemas"]["TeacherProfileWritable"];

/** The editable slice of a profile, as the dashboard form holds it. */
export interface ProfileFormValues {
  displayName: string;
  headline: string;
  kind: "professional" | "community";
  countryCode: string;
  countryName: string;
  city: string;
  timezone: string;
  pricePerHourMinor: number;
  trialPriceMinor: number | null;
  about: string;
  teachingStyle: string;
  /** Current media as displayable URLs (read-only here — the writable side
   *  is the asset id; a seed teacher's pasted photo shows until replaced). */
  avatarUrl: string;
  introVideoUrl: string;
  /** Uploads chosen in the media step; null = none / removed. */
  avatarAssetId: string | null;
  introVideoAssetId: string | null;
  meetingUrl: string;
  languages: {
    role: "teaches" | "also_speaks";
    code: string;
    name: string;
    level: components["schemas"]["LanguageLevel"];
  }[];
  focus: string[];
  experience: { title: string; org: string; period: string }[];
}

export class ProfileError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ProfileError";
    this.code = code;
    this.status = status;
  }
}

type ErrorBody = { error?: { code?: string; message?: string } };

function toError(error: unknown, status: number, fallback: string): ProfileError {
  const body = error as ErrorBody | undefined;
  return new ProfileError(
    body?.error?.message ?? fallback,
    body?.error?.code ?? "unknown",
    status,
  );
}

function toWritable(v: ProfileFormValues): Writable {
  return {
    display_name: v.displayName,
    headline: v.headline,
    kind: v.kind,
    country_code: v.countryCode,
    country_name: v.countryName,
    city: v.city,
    timezone: v.timezone,
    price_per_hour_minor: v.pricePerHourMinor,
    trial_price_minor: v.trialPriceMinor,
    currency: "UZS",
    about: v.about,
    teaching_style: v.teachingStyle,
    // Media is written as asset ids only; the backend derives the URLs
    // (and clears them on an explicit null), so the *_url strings never
    // round-trip from the form.
    avatar_asset_id: v.avatarAssetId,
    intro_video_asset_id: v.introVideoAssetId,
    meeting_url: v.meetingUrl,
    languages: v.languages,
    focus: v.focus,
    experience: v.experience,
  };
}

/** The caller's own profile, or null if they haven't created one yet. */
export async function getMyProfile(): Promise<TeacherProfile | null> {
  const { data, error, response } = await browserApi.GET("/v1/teachers/me", {});
  if (response.status === 404) return null;
  if (error || !data) {
    throw toError(error, response.status, "Could not load your teacher profile.");
  }
  return toProfile(data);
}

export async function createMyProfile(
  values: ProfileFormValues,
): Promise<TeacherProfile> {
  const { data, error, response } = await browserApi.POST("/v1/teachers", {
    body: toWritable(values),
  });
  if (error || !data) {
    throw toError(error, response.status, "Could not create your profile.");
  }
  return toProfile(data);
}

export async function updateMyProfile(
  slug: string,
  values: ProfileFormValues,
): Promise<TeacherProfile> {
  const { data, error, response } = await browserApi.PATCH("/v1/teachers/{slug}", {
    params: { path: { slug } },
    body: toWritable(values),
  });
  if (error || !data) {
    throw toError(error, response.status, "Could not save your changes.");
  }
  return toProfile(data);
}

/** Seed a blank form (used for the "create profile" state). */
export function emptyProfileForm(displayName = ""): ProfileFormValues {
  return {
    displayName,
    headline: "",
    kind: "community",
    countryCode: "UZ",
    countryName: "Uzbekistan",
    city: "",
    timezone: "Asia/Tashkent",
    pricePerHourMinor: 0,
    trialPriceMinor: null,
    about: "",
    teachingStyle: "",
    avatarUrl: "",
    introVideoUrl: "",
    avatarAssetId: null,
    introVideoAssetId: null,
    meetingUrl: "",
    languages: [],
    focus: [],
    experience: [],
  };
}

/** Populate the form from an existing profile. */
export function profileToForm(p: TeacherProfile): ProfileFormValues {
  return {
    displayName: p.displayName,
    headline: p.headline,
    kind: p.kind,
    countryCode: p.countryCode,
    countryName: p.countryName,
    city: p.city,
    timezone: p.timezone,
    pricePerHourMinor: p.pricePerHour.amountMinor,
    trialPriceMinor: p.trialPrice?.amountMinor ?? null,
    about: p.about,
    teachingStyle: p.teachingStyle,
    avatarUrl: p.avatarUrl,
    introVideoUrl: p.introVideoUrl,
    avatarAssetId: p.avatarAssetId,
    introVideoAssetId: p.introVideoAssetId,
    meetingUrl: p.meetingUrl,
    languages: [
      ...p.teaches.map((l) => ({
        role: "teaches" as const,
        code: l.code,
        name: l.name,
        level: l.level,
      })),
      ...p.alsoSpeaks.map((l) => ({
        role: "also_speaks" as const,
        code: l.code,
        name: l.name,
        level: l.level,
      })),
    ],
    focus: p.focus,
    experience: p.experience,
  };
}

/* ----------------------------- lesson types ------------------------------ */


type WireLessonType = components["schemas"]["LessonType"];

function toLessonTypeVM(w: WireLessonType): LessonType {
  return {
    id: w.id,
    title: w.title,
    description: w.description,
    isTrial: w.is_trial,
    archived: w.archived,
    position: w.position,
    from: toMoney(w.from),
    prices: w.prices.map((p) => ({
      durationMinutes: p.duration_minutes as LessonType["prices"][number]["durationMinutes"],
      price: toMoney(p.price),
    })),
  };
}

/** The lengths the API prices an offering at. */
export type LessonDuration = 30 | 45 | 60 | 90 | 120;

export const LESSON_DURATIONS: readonly LessonDuration[] = [30, 45, 60, 90, 120];

export interface LessonTypeDraft {
  title: string;
  description: string;
  isTrial?: boolean;
  position?: number;
  prices: { durationMinutes: LessonDuration; priceMinor: number }[];
}

function toBody(draft: LessonTypeDraft) {
  return {
    title: draft.title,
    description: draft.description,
    is_trial: draft.isTrial ?? false,
    position: draft.position ?? 0,
    prices: draft.prices.map((p) => ({
      duration_minutes: p.durationMinutes,
      price_minor: p.priceMinor,
    })),
  };
}

/** The caller's own offerings, archived ones included. */
export async function getOwnLessonTypes(): Promise<LessonType[]> {
  const { data, error, response } = await browserApi.GET("/v1/teachers/me/lesson-types", {});
  if (error || !data) {
    if (response.status === 404) return [];
    throw new ProfileError("Could not load your lessons.", "unknown", response.status);
  }
  return data.lesson_types.map(toLessonTypeVM);
}

export async function createLessonType(draft: LessonTypeDraft): Promise<LessonType> {
  const { data, error, response } = await browserApi.POST("/v1/teachers/me/lesson-types", {
    body: toBody(draft),
  });
  if (error || !data) throw toError(error, response.status, "Could not save that lesson.");
  return toLessonTypeVM(data);
}

export async function updateLessonType(id: string, draft: LessonTypeDraft): Promise<LessonType> {
  const { data, error, response } = await browserApi.PATCH("/v1/teachers/me/lesson-types/{id}", {
    params: { path: { id } },
    body: toBody(draft),
  });
  if (error || !data) throw toError(error, response.status, "Could not save that lesson.");
  return toLessonTypeVM(data);
}

export async function setLessonTypeArchived(id: string, archived: boolean): Promise<LessonType> {
  const path = archived
    ? ("/v1/teachers/me/lesson-types/{id}/archive" as const)
    : ("/v1/teachers/me/lesson-types/{id}/restore" as const);
  const { data, error, response } = await browserApi.POST(path, {
    params: { path: { id } },
  });
  if (error || !data) throw toError(error, response.status, "Could not update that lesson.");
  return toLessonTypeVM(data);
}
