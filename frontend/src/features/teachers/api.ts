import type { TeacherProfile, TeacherSummary } from "@/types/teacher";
import { MOCK_TEACHERS } from "./mock-data";

/**
 * Data-access layer for the teachers module.
 *
 * Right now every function reads from in-memory mock data. The signatures are
 * deliberately async and return plain data (no framework objects), so when the
 * generated `openapi-fetch` client arrives we only change the bodies — callers
 * (server components, later hooks) don't change.
 */

export interface TeacherListParams {
  /** filter by taught-language code, e.g. "es" */
  language?: string;
  kind?: TeacherProfile["kind"];
  /** max price per hour in minor units */
  maxPriceMinor?: number;
  /** substring match on name / headline / focus */
  q?: string;
  sort?: TeacherSort;
}

export type TeacherSort = "recommended" | "price_asc" | "price_desc" | "rating_desc";

export interface TeacherListResult {
  teachers: TeacherSummary[];
  total: number;
  /** languages present in the full catalog, for building the filter UI */
  facets: {
    languages: { code: string; name: string; count: number }[];
  };
}

/** Simulate network latency so loading states are visible in dev. */
const delay = (ms = 150) => new Promise((r) => setTimeout(r, ms));

export async function listTeachers(
  params: TeacherListParams = {},
): Promise<TeacherListResult> {
  await delay();

  const languages = buildLanguageFacets(MOCK_TEACHERS);

  let rows = MOCK_TEACHERS.slice();

  if (params.language) {
    rows = rows.filter((t) => t.teaches.some((l) => l.code === params.language));
  }
  if (params.kind) {
    rows = rows.filter((t) => t.kind === params.kind);
  }
  if (params.maxPriceMinor != null) {
    rows = rows.filter((t) => t.pricePerHour.amountMinor <= params.maxPriceMinor!);
  }
  if (params.q) {
    const needle = params.q.toLowerCase();
    rows = rows.filter((t) =>
      [t.displayName, t.headline, ...t.focus]
        .join(" ")
        .toLowerCase()
        .includes(needle),
    );
  }

  rows = sortTeachers(rows, params.sort ?? "recommended");

  // The mock returns full profiles; TeacherProfile extends TeacherSummary, so
  // this is a safe widening. The real /teachers endpoint returns real summaries.
  return {
    teachers: rows,
    total: rows.length,
    facets: { languages },
  };
}

export async function getTeacherBySlug(slug: string): Promise<TeacherProfile | null> {
  await delay();
  return MOCK_TEACHERS.find((t) => t.slug === slug) ?? null;
}

/** For generateStaticParams — the set of profile pages to prerender. */
export async function listTeacherSlugs(): Promise<string[]> {
  return MOCK_TEACHERS.map((t) => t.slug);
}

function buildLanguageFacets(teachers: TeacherProfile[]) {
  const map = new Map<string, { code: string; name: string; count: number }>();
  for (const t of teachers) {
    for (const l of t.teaches) {
      const entry = map.get(l.code) ?? { code: l.code, name: l.name, count: 0 };
      entry.count += 1;
      map.set(l.code, entry);
    }
  }
  return [...map.values()].sort((a, b) => a.name.localeCompare(b.name));
}

function sortTeachers(rows: TeacherProfile[], sort: TeacherSort): TeacherProfile[] {
  const byRecommended = (a: TeacherProfile, b: TeacherProfile) =>
    Number(b.acceptingStudents) - Number(a.acceptingStudents) ||
    b.rating - a.rating ||
    b.reviewCount - a.reviewCount;

  switch (sort) {
    case "price_asc":
      return rows.sort((a, b) => a.pricePerHour.amountMinor - b.pricePerHour.amountMinor);
    case "price_desc":
      return rows.sort((a, b) => b.pricePerHour.amountMinor - a.pricePerHour.amountMinor);
    case "rating_desc":
      return rows.sort((a, b) => b.rating - a.rating || b.reviewCount - a.reviewCount);
    case "recommended":
    default:
      return rows.sort(byRecommended);
  }
}
