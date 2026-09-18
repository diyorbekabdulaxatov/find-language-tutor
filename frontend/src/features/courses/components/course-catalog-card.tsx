import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Clock, Film, GraduationCap, Layers, PlayCircle } from "lucide-react";
import { courseLengthParts, formatMoney } from "@/lib/format";
import { courseCoverUrl } from "@/features/courses/api";
import type { CourseCatalogEntry } from "@/features/courses/types";
import { Stars } from "@/features/reviews/components/star-rating";

/**
 * Grid card for the public course catalog. Mirrors `TeacherCard`'s
 * hover/shadow language, swapped to a video-course cover instead of a photo —
 * the whole card is a link, price + curriculum size sit on a footer row.
 */
export function CourseCatalogCard({ course }: { course: CourseCatalogEntry }) {
  const free = course.price.amountMinor === 0;
  const t = useTranslations("courses");
  const locale = useLocale();
  // null when no item reported a length — the row is omitted rather than
  // rendered as "0m", so a missing figure never reads as an empty course.
  const length = courseLengthParts(course.totalDurationSeconds);

  return (
    <Link
      href={`/courses/catalog/${course.id}`}
      className="group flex flex-col overflow-hidden rounded-2xl bg-card ring-1 ring-border shadow-card transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      <div className="relative aspect-[16/10] overflow-hidden bg-muted">
        {course.coverAssetId ? (
          // Public, unauthenticated endpoint — a plain <img> works from a
          // server-rendered card with no client-side fetch needed.
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={courseCoverUrl(course.id)}
            alt=""
            className="size-full object-cover transition-transform duration-500 group-hover:scale-[1.04]"
          />
        ) : (
          <div className="grid size-full place-items-center text-muted-foreground">
            <Film className="size-8" />
          </div>
        )}

        {free && (
          <span className="absolute left-3 top-3 rounded-full bg-mint/90 px-2.5 py-1 text-xs font-semibold text-mint-foreground shadow-soft backdrop-blur">
            {t("free")}
          </span>
        )}

        {course.hasPreview && (
          <span className="absolute right-3 top-3 inline-flex items-center gap-1 rounded-full bg-background/90 px-2.5 py-1 text-xs font-semibold text-foreground shadow-soft backdrop-blur">
            <PlayCircle className="size-3.5" />
            {t("freePreviewBadge")}
          </span>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-2.5 p-4">
        <h3 className="font-display text-lg leading-tight text-foreground">
          {course.title}
        </h3>

        {course.subtitle && (
          <p className="line-clamp-2 text-sm leading-snug text-muted-foreground">
            {course.subtitle}
          </p>
        )}

        <p className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
          <GraduationCap className="size-3.5" />
          {course.teacher.displayName}
        </p>

        {course.reviewCount > 0 && (
          <p className="inline-flex items-center gap-1.5 text-xs">
            <span className="font-semibold text-foreground">{course.rating.toFixed(1)}</span>
            <Stars value={course.rating} className="[&_svg]:size-3.5" />
            <span className="text-muted-foreground">({course.reviewCount})</span>
          </p>
        )}

        <p className="inline-flex flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-muted-foreground">
          <Layers className="size-3.5" />
          {t("sections", { count: course.sectionCount })} · {t("lessons", { count: course.itemCount })}
          {length && (
            <>
              <span aria-hidden>·</span>
              <Clock className="size-3.5" />
              {length.hours > 0
                ? t("courseLengthHm", { hours: length.hours, minutes: length.minutes })
                : t("courseLengthM", { minutes: length.minutes })}
            </>
          )}
        </p>

        <div className="mt-auto flex items-center justify-between gap-2 border-t border-border pt-3">
          <span className="font-display text-base text-foreground">
            {free ? t("free") : formatMoney(course.price, locale)}
          </span>
          <span className="rounded-lg bg-primary px-3.5 py-2 text-sm font-semibold text-primary-foreground transition-colors group-hover:bg-primary/90">
            {t("viewCourse")}
          </span>
        </div>
      </div>
    </Link>
  );
}
