import type { Metadata } from "next";
import { listCourseCatalog } from "@/features/courses/api";
import type { CourseSort } from "@/features/courses/types";
import { CourseCatalogCard } from "@/features/courses/components/course-catalog-card";
import { CourseCatalogFilters } from "@/features/courses/components/course-catalog-filters";

export const metadata: Metadata = {
  title: "Find a course",
  description:
    "Browse self-paced video courses from FindTutor's teachers — watch on your own schedule, with quizzes and homework along the way.",
};

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
  const sort = (first(sp.sort) as CourseSort | undefined) ?? undefined;

  const { courses, total } = await listCourseCatalog({ q, sort, pageSize: PAGE_SIZE });

  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:py-14">
      <header className="max-w-2xl">
        <h1 className="font-display text-3xl tracking-tight sm:text-4xl">
          Learn at your own pace
        </h1>
        <p className="mt-3 text-muted-foreground">
          Self-paced video courses built by FindTutor&rsquo;s teachers — watch
          lessons, practice with quizzes and homework, whenever you have time.
        </p>
      </header>

      <div className="mt-8">
        <CourseCatalogFilters />
      </div>

      <p className="mt-6 text-sm text-muted-foreground">
        <span className="font-semibold text-foreground">{total}</span>{" "}
        {total === 1 ? "course" : "courses"} available
      </p>

      {courses.length === 0 ? (
        <div className="mt-6 rounded-2xl border border-dashed border-border bg-card p-12 text-center">
          <p className="font-display text-xl">No courses match that search</p>
          <p className="mt-2 text-sm text-muted-foreground">
            Try a different keyword, or clear your search.
          </p>
        </div>
      ) : (
        <div className="mt-4 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {courses.map((course) => (
            <CourseCatalogCard key={course.id} course={course} />
          ))}
        </div>
      )}
    </div>
  );
}
