import type { Metadata } from "next";
import { listTeachers, type TeacherListParams, type TeacherSort } from "@/features/teachers/api";
import { TeacherFilters } from "@/features/teachers/components/teacher-filters";
import { TeacherListItem } from "@/features/teachers/components/teacher-list-item";

export const metadata: Metadata = {
  title: "Find a teacher",
  description:
    "Browse English, Russian, and more language teachers. Filter by language, price, and teaching style, then book a 1-on-1 lesson.",
};

/**
 * Server component. Reading `searchParams` opts this route into dynamic
 * rendering, which is what we want for a search page — the results depend on the
 * request. Profile pages stay static (see teachers/[slug]).
 *
 * In Next 16 `searchParams` is a Promise, so it has to be awaited.
 */
export default async function TeachersPage({
  searchParams,
}: PageProps<"/teachers">) {
  const sp = await searchParams;
  const params = parseParams(sp);
  const { teachers, total, facets } = await listTeachers(params);

  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:py-14">
      <header className="max-w-2xl">
        <h1 className="font-display text-3xl font-medium tracking-tight sm:text-4xl">
          Find a teacher
        </h1>
        <p className="mt-3 text-muted-foreground">
          Every teacher here gives paid 1-on-1 video lessons. Watch a few intros,
          book a trial, and keep the one you click with.
        </p>
      </header>

      <div className="mt-10 grid gap-10 lg:grid-cols-[240px_1fr]">
        <aside className="lg:sticky lg:top-20 lg:self-start">
          <TeacherFilters facets={facets} />
        </aside>

        <section>
          <p className="text-sm text-muted-foreground">
            {total} {total === 1 ? "teacher" : "teachers"}
          </p>

          {teachers.length === 0 ? (
            <div className="mt-10 rounded-xl border border-dashed border-border p-10 text-center">
              <p className="font-display text-lg">No teachers match those filters</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Try widening the price range or choosing a different language.
              </p>
            </div>
          ) : (
            <div className="mt-2">
              {teachers.map((teacher) => (
                <TeacherListItem key={teacher.id} teacher={teacher} />
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}

type RawSearchParams = { [key: string]: string | string[] | undefined };

/** URL params -> the typed shape the data layer expects. */
function parseParams(sp: RawSearchParams): TeacherListParams {
  const first = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);

  const maxSom = Number(first(sp.max));
  const sort = first(sp.sort) as TeacherSort | undefined;

  return {
    language: first(sp.lang) || undefined,
    kind: (first(sp.kind) as TeacherListParams["kind"]) || undefined,
    // URL carries whole so'm; the data layer works in minor units.
    maxPriceMinor: Number.isFinite(maxSom) && maxSom > 0 ? maxSom * 100 : undefined,
    q: first(sp.q) || undefined,
    sort: sort ?? undefined,
  };
}
