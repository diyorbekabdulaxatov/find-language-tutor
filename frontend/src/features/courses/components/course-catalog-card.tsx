import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Film } from "lucide-react";
import { cn } from "@/lib/utils";
import { courseLengthParts, formatMoney } from "@/lib/format";
import { courseCoverUrl } from "@/features/courses/api";
import type { CourseCatalogEntry } from "@/features/courses/types";
import { Stars } from "@/features/reviews/components/star-rating";

function CourseCover({ course, className }: { course: CourseCatalogEntry; className?: string }) {
  return (
    <div className={cn("relative overflow-hidden border border-border bg-muted", className)}>
      {course.coverAssetId ? (
        // Public, unauthenticated endpoint — a plain <img> works from a
        // server-rendered card with no client-side fetch needed.
        // eslint-disable-next-line @next/next/no-img-element
        <img src={courseCoverUrl(course.id)} alt="" className="size-full object-cover" />
      ) : (
        <div className="grid size-full place-items-center text-muted-foreground">
          <Film className="size-8" />
        </div>
      )}
    </div>
  );
}

function useCourseMeta(course: CourseCatalogEntry) {
  const t = useTranslations("courses");
  const length = courseLengthParts(course.totalDurationSeconds);
  const lengthLabel = length
    ? length.hours > 0
      ? t("totalHours", { hours: length.hours + (length.minutes >= 30 ? 0.5 : 0) })
      : t("totalMinutes", { minutes: length.minutes })
    : null;
  return { t, lengthLabel };
}

/**
 * Udemy's course card: flat cover, bold two-line title, grey by-line, the
 * rating row (brown number, amber stars, grey count), the price in bold and
 * a "Bestseller"-style badge. Used on the home page's featured row.
 */
export function CourseCatalogCard({ course }: { course: CourseCatalogEntry }) {
  const free = course.price.amountMinor === 0;
  const locale = useLocale();
  const { t, lengthLabel } = useCourseMeta(course);

  return (
    <Link
      href={`/courses/catalog/${course.id}`}
      className="group flex flex-col outline-none focus-visible:ring-3 focus-visible:ring-ring/40"
    >
      <CourseCover course={course} className="aspect-[16/9]" />
      <div className="flex flex-1 flex-col gap-1 pt-2">
        <h3 className="line-clamp-2 text-base leading-tight font-bold text-foreground group-hover:text-link">
          {course.title}
        </h3>
        <p className="line-clamp-1 text-xs text-muted-foreground">{course.teacher.displayName}</p>
        {course.reviewCount > 0 && (
          <p className="inline-flex items-center gap-1 text-xs">
            <span className="font-bold text-rating">{course.rating.toFixed(1)}</span>
            <Stars value={course.rating} className="[&_svg]:size-3.5" />
            <span className="text-muted-foreground">({course.reviewCount})</span>
          </p>
        )}
        <p className="text-xs text-muted-foreground">
          {[lengthLabel, t("lectures", { count: course.itemCount })].filter(Boolean).join(" · ")}
        </p>
        <p className="text-base font-bold text-foreground">
          {free ? t("free") : formatMoney(course.price, locale)}
        </p>
        <div className="flex flex-wrap gap-1.5">
          {course.hasPreview && (
            <span className="bg-[#eceb98] px-2 py-0.5 text-xs font-bold text-[#3d3c0a]">
              {t("freePreviewBadge")}
            </span>
          )}
        </div>
      </div>
    </Link>
  );
}

/**
 * Udemy's search-result row: cover on the left, title / subtitle / by-line /
 * rating / length in the middle, price on the right.
 */
export function CourseCatalogRow({ course }: { course: CourseCatalogEntry }) {
  const free = course.price.amountMinor === 0;
  const locale = useLocale();
  const { t, lengthLabel } = useCourseMeta(course);

  return (
    <Link
      href={`/courses/catalog/${course.id}`}
      className="group flex gap-4 border-b border-border py-4 outline-none focus-visible:ring-3 focus-visible:ring-ring/40"
    >
      <CourseCover course={course} className="aspect-[16/9] w-32 shrink-0 sm:w-64" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-start justify-between gap-4">
          <h3 className="line-clamp-2 text-base leading-tight font-bold text-foreground group-hover:text-link">
            {course.title}
          </h3>
          <p className="shrink-0 text-base font-bold text-foreground">
            {free ? t("free") : formatMoney(course.price, locale)}
          </p>
        </div>
        {course.subtitle && (
          <p className="line-clamp-2 text-sm text-foreground/90">{course.subtitle}</p>
        )}
        <p className="text-xs text-muted-foreground">{course.teacher.displayName}</p>
        {course.reviewCount > 0 && (
          <p className="inline-flex items-center gap-1 text-xs">
            <span className="font-bold text-rating">{course.rating.toFixed(1)}</span>
            <Stars value={course.rating} className="[&_svg]:size-3.5" />
            <span className="text-muted-foreground">({course.reviewCount})</span>
          </p>
        )}
        <p className="text-xs text-muted-foreground">
          {[
            lengthLabel,
            t("lectures", { count: course.itemCount }),
            t("sections", { count: course.sectionCount }),
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
        {course.hasPreview && (
          <span className="mt-1 w-fit bg-[#eceb98] px-2 py-0.5 text-xs font-bold text-[#3d3c0a]">
            {t("freePreviewBadge")}
          </span>
        )}
      </div>
    </Link>
  );
}
