import type { TeacherProfile, TeacherSummary, Money } from "@/types/teacher";
import { api, type ApiSchemas } from "@/lib/api/client";

/**
 * Data-access layer for the teachers module.
 *
 * Talks to the Go backend through the generated `openapi-fetch` client. The
 * wire format is snake_case (see openapi.yaml); the mappers at the bottom turn
 * each response into the camelCase view-models in `@/types/teacher` that the
 * components consume. Callers (server components) only ever see the view-models.
 */

export type TeacherSort = ApiSchemas["TeacherSort"];

export interface TeacherListParams {
  /** filter by taught-language code, e.g. "en" */
  language?: string;
  kind?: TeacherSummary["kind"];
  /** max price per hour in minor units */
  maxPriceMinor?: number;
  /** substring match on name / headline / focus */
  q?: string;
  sort?: TeacherSort;
}

export interface TeacherListResult {
  teachers: TeacherSummary[];
  total: number;
  /** languages present in the full catalog, for building the filter UI */
  facets: {
    languages: { code: string; name: string; count: number }[];
  };
}

/**
 * No pagination UI exists yet, so ask for a page big enough to hold the whole
 * catalog and keep the "show everything that matches" behaviour.
 */
const LIST_PAGE_SIZE = 100;

export async function listTeachers(
  params: TeacherListParams = {},
): Promise<TeacherListResult> {
  const { data, error } = await api.GET("/v1/teachers", {
    params: {
      query: {
        language: params.language,
        kind: params.kind,
        max_price_minor: params.maxPriceMinor,
        q: params.q,
        sort: params.sort,
        page_size: LIST_PAGE_SIZE,
      },
    },
  });

  if (error || !data) {
    throw new Error(`Failed to load teachers: ${describeError(error)}`);
  }

  return {
    teachers: data.teachers.map(toSummary),
    total: data.total,
    facets: {
      languages: data.facets.languages.map((l) => ({
        code: l.code,
        name: l.name,
        count: l.count,
      })),
    },
  };
}

export async function getTeacherBySlug(
  slug: string,
): Promise<TeacherProfile | null> {
  const { data, error, response } = await api.GET("/v1/teachers/{slug}", {
    params: { path: { slug } },
  });

  if (response.status === 404) return null;
  if (error || !data) {
    throw new Error(`Failed to load teacher "${slug}": ${describeError(error)}`);
  }

  return toProfile(data);
}

/** For generateStaticParams — the set of profile pages to prerender. */
export async function listTeacherSlugs(): Promise<string[]> {
  try {
    const { data, error } = await api.GET("/v1/teachers", {
      params: { query: { page_size: LIST_PAGE_SIZE } },
    });

    // Don't fail the build if the backend is unreachable (error return) or
    // down entirely (fetch throws) — pages still render on demand because
    // `dynamicParams` stays true.
    if (error || !data) return [];

    return data.teachers.map((t) => t.slug);
  } catch {
    return [];
  }
}

/* -------------------------------------------------------------------------- */
/* wire (snake_case) -> view-model (camelCase)                                */
/* -------------------------------------------------------------------------- */

export function toMoney(m: ApiSchemas["Money"]): Money {
  return { amountMinor: m.amount_minor, currency: m.currency };
}

export function toSummary(t: ApiSchemas["TeacherSummary"]): TeacherSummary {
  return {
    id: t.id,
    slug: t.slug,
    displayName: t.display_name,
    headline: t.headline,
    avatarUrl: t.avatar_url,
    videoThumbnailUrl: t.video_thumbnail_url,
    kind: t.kind,
    countryCode: t.country_code,
    countryName: t.country_name,
    city: t.city,
    timezone: t.timezone,
    teaches: t.teaches,
    alsoSpeaks: t.also_speaks,
    pricePerHour: toMoney(t.price_per_hour),
    rating: t.rating,
    reviewCount: t.review_count,
    lessonsCompleted: t.lessons_completed,
    studentCount: t.student_count,
    focus: t.focus,
    responseTimeHours: t.response_time_hours,
    acceptingStudents: t.accepting_students,
  };
}

export function toProfile(t: ApiSchemas["TeacherProfile"]): TeacherProfile {
  return {
    ...toSummary(t),
    introVideoUrl: t.intro_video_url,
    about: t.about,
    teachingStyle: t.teaching_style,
    experience: t.experience,
    trialPrice: t.trial_price ? toMoney(t.trial_price) : undefined,
  };
}

function describeError(error: unknown): string {
  if (error && typeof error === "object" && "error" in error) {
    const inner = (error as { error?: { message?: string } }).error;
    if (inner?.message) return inner.message;
  }
  return "unknown error";
}
