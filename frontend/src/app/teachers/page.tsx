import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { listTeachers, type TeacherListParams, type TeacherSort } from "@/features/teachers/api";
import { TeacherFilters } from "@/features/teachers/components/teacher-filters";
import { TeacherRow } from "@/features/teachers/components/teacher-card";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("teachersPage");
  return { title: t("metaTitle"), description: t("metaDescription") };
}

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
  const [t, { teachers, total, facets }] = await Promise.all([
    getTranslations("teachersPage"),
    listTeachers(params),
  ]);

  return (
    <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6 lg:py-10">
      <h1 className="font-display text-3xl sm:text-[2rem]">
        {sp.q ? t("resultsFor", { q: String(sp.q) }) : t("title")}
      </h1>
      <p className="mt-2 max-w-2xl text-base text-muted-foreground">{t("intro")}</p>

      <div className="mt-8">
        <TeacherFilters facets={facets} total={total}>
          {teachers.length === 0 ? (
            <div className="border border-border px-6 py-16 text-center">
              <p className="text-xl font-bold">{t("noMatchTitle")}</p>
              <p className="mt-2 text-sm text-muted-foreground">{t("noMatchBody")}</p>
            </div>
          ) : (
            <div className="border-t border-border">
              {teachers.map((teacher) => (
                <TeacherRow key={teacher.id} teacher={teacher} />
              ))}
            </div>
          )}
        </TeacherFilters>
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
