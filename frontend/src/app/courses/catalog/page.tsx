import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { listCourseCatalog } from "@/features/courses/api";
import type { CourseSort } from "@/features/courses/types";
import { CourseCatalogRow } from "@/features/courses/components/course-catalog-card";
import { CourseCatalogLayout } from "@/features/courses/components/course-catalog-filters";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaCatalog"), description: t("metaCatalogDescription") };
}

const PAGE_SIZE = 24;

/**
 * Server component, public catalog. Reading `searchParams` opts this route
 * into dynamic rendering — the results depend on the request, same reasoning
 * as `/teachers`.
 */
export default async function CourseCatalogPage({
  searchParams,
}: PageProps<"/courses/catalog">) {
  const sp = await searchParams;
  const first = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v);

  const q = first(sp.q) || undefined;
  const sort = (first(sp.sort) as CourseSort | undefined) ?? "rating";
  const maxPrice = first(sp.max_price);
  const maxPriceMinor = maxPrice != null && maxPrice !== "" ? Number(maxPrice) : undefined;

  const [t, { courses, total }] = await Promise.all([
    getTranslations("courses"),
    listCourseCatalog({ q, sort, maxPriceMinor, pageSize: PAGE_SIZE }),
  ]);

  return (
    <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6 lg:py-10">
      <h1 className="font-display text-3xl sm:text-[2rem]">{t("catalogTitle")}</h1>
      <p className="mt-2 max-w-2xl text-base text-muted-foreground">{t("catalogIntro")}</p>

      <div className="mt-8">
        <CourseCatalogLayout total={total}>
          {courses.length === 0 ? (
            <div className="border border-border px-6 py-16 text-center">
              <p className="text-xl font-bold">{t("noMatchTitle")}</p>
              <p className="mt-2 text-sm text-muted-foreground">{t("noMatchBody")}</p>
            </div>
          ) : (
            <div className="border-t border-border">
              {courses.map((course) => (
                <CourseCatalogRow key={course.id} course={course} />
              ))}
            </div>
          )}
        </CourseCatalogLayout>
      </div>
    </div>
  );
}
