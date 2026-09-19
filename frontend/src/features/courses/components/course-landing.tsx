"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import {
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  FileText,
  Film,
  Globe,
  Infinity as InfinityIcon,
  MonitorSmartphone,
  PlayCircle,
  PlaySquare,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { courseLengthParts, formatLectureLength, formatMoney } from "@/lib/format";
import { useAuth } from "@/features/auth/auth-context";
import { refreshSession } from "@/features/auth/browser-client";
import {
  courseCoverUrl,
  coursePreviewUrl,
  getMyCourseCatalogDetail,
} from "@/features/courses/api";
import type { CourseCatalogDetail, CourseEnrollment } from "@/features/courses/types";
import { PurchaseCoursePanel } from "./purchase-course-panel";
import { CourseReviews } from "./course-reviews";
import { Stars } from "@/features/reviews/components/star-rating";

/**
 * The public course landing page. Fetches through `browserApi` (via
 * `getMyCourseCatalogDetail`) so a signed-in viewer's bearer token — if any —
 * personalises `isEnrolled` / `isOwner`; the same call works logged out
 * (both read false). Client-rendered rather than split server/client since no
 * SSR-personalization convention exists elsewhere in the app (teacher
 * profiles carry no such "is this mine" state); `/courses/catalog/[id]/page.tsx`
 * still generates SEO metadata server-side from the unauthenticated read.
 *
 * This is the app's first *optional*-auth endpoint: a stale-but-present
 * access token isn't rejected with a 401 (auth is optional, so the backend
 * just treats it as absent), which means `browserApi`'s usual "refresh once
 * on 401" never fires here — unlike every other authed call in the app, this
 * one can silently under-personalise if the 15-minute access token expired
 * since the last refresh. Waiting for the auth bootstrap and proactively
 * refreshing first (only when a session exists) keeps the token fresh before
 * the personalised fetch.
 */
export function CourseLanding({ id }: { id: string }) {
  const { status } = useAuth();
  const [course, setCourse] = useState<CourseCatalogDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "not-found" | "error">("loading");
  const [justEnrolled, setJustEnrolled] = useState<CourseEnrollment | null>(null);
  // Which free preview lecture is playing in the hero, if any. Clearing it
  // puts the cover image back.
  const [previewItemId, setPreviewItemId] = useState<string | null>(null);
  const [expandedAll, setExpandedAll] = useState(false);
  const [descriptionOpen, setDescriptionOpen] = useState(false);
  const t = useTranslations("courses");
  const locale = useLocale();

  useEffect(() => {
    if (status === "loading") return;
    let alive = true;
    async function load() {
      try {
        if (status === "authenticated") await refreshSession();
        const c = await getMyCourseCatalogDetail(id);
        if (!alive) return;
        if (!c) {
          setState("not-found");
          return;
        }
        setCourse(c);
        setState("ready");
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id, status]);

  if (state === "loading") {
    return (
      <div>
        <div className="h-64 bg-ink" />
        <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6">
          <div className="h-96 max-w-[700px] animate-pulse bg-muted" />
        </div>
      </div>
    );
  }

  if (state === "not-found" || !course) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
        <p className="font-display text-2xl">{t("notFound")}</p>
        <p className="mt-2 text-sm text-muted-foreground">{t("notFoundBody")}</p>
        <Button asChild className="mt-6">
          <Link href="/courses/catalog">{t("browseCourses")}</Link>
        </Button>
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoad")}
        </p>
      </div>
    );
  }

  const free = course.price.amountMinor === 0;
  const allItems = course.sections.flatMap((s) => s.items);
  const totalItems = allItems.length;
  const videoCount = allItems.filter((i) => i.kind === "video").length;
  const resourceCount = totalItems - videoCount;
  const length = courseLengthParts(course.totalDurationSeconds);
  const lengthLabel = length
    ? length.hours > 0
      ? t("totalHours", { hours: length.hours + (length.minutes >= 30 ? 0.5 : 0) })
      : t("totalMinutes", { minutes: length.minutes })
    : null;
  // The first free lecture in curriculum order — what the sidebar's play
  // button starts with. null when the teacher has not marked any.
  const firstPreview = allItems.find((i) => i.isPreview) ?? null;
  const enrolled = course.isEnrolled || justEnrolled !== null;

  const cover = (
    <div className="relative aspect-video bg-muted">
      {previewItemId ? (
        <video
          key={previewItemId}
          controls
          autoPlay
          src={coursePreviewUrl(course.id, previewItemId)}
          className="size-full bg-black object-contain"
        />
      ) : course.coverAssetId ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={courseCoverUrl(course.id)} alt="" className="size-full object-cover" />
      ) : (
        <div className="grid size-full place-items-center text-muted-foreground">
          <Film className="size-10" />
        </div>
      )}
      {/* The cover doubles as the play surface when a free sample exists —
          the "try before you buy" affordance shoppers look for first. */}
      {!previewItemId && firstPreview && (
        <button
          type="button"
          onClick={() => setPreviewItemId(firstPreview.id)}
          className="absolute inset-0 grid place-items-center bg-gradient-to-t from-black/70 to-black/10 text-white outline-none focus-visible:ring-3 focus-visible:ring-ring/60"
        >
          <span className="grid size-16 place-items-center rounded-full bg-white text-ink shadow-lift">
            <PlayCircle className="size-8" />
          </span>
          <span className="absolute inset-x-0 bottom-3 text-center text-base font-bold">
            {t("previewThisCourse")}
          </span>
        </button>
      )}
    </div>
  );

  const buyCard = (
    <div className="bg-card text-card-foreground shadow-card lg:border lg:border-border">
      <div className="hidden lg:block">{cover}</div>
      <div className="p-6">
        <p className="font-display text-[2rem] leading-none text-foreground">
          {free ? t("free") : formatMoney(course.price, locale)}
        </p>

        <div className="mt-4">
          {course.isOwner ? (
            <Button asChild size="lg" className="w-full">
              <Link href={`/courses/${course.id}/edit`}>{t("editCourse")}</Link>
            </Button>
          ) : enrolled ? (
            <div className="flex flex-col gap-3">
              {justEnrolled && (
                <p className="inline-flex items-center gap-2 bg-accent px-3 py-2 text-sm font-bold text-accent-foreground">
                  <CheckCircle2 className="size-4" />
                  {t("youreEnrolled")}
                </p>
              )}
              <Button asChild size="lg" className="w-full">
                <Link href={`/learn/${course.id}`}>{t("continueLearning")}</Link>
              </Button>
            </div>
          ) : (
            <PurchaseCoursePanel
              courseId={course.id}
              price={course.price}
              onPurchased={setJustEnrolled}
            />
          )}
        </div>

        {!course.isOwner && (
          <p className="mt-3 text-center text-xs text-muted-foreground">{t("lifetimeAccess")}</p>
        )}

        <div className="mt-6">
          <p className="text-base font-bold">{t("thisCourseIncludes")}</p>
          <ul className="mt-2 space-y-1.5 text-sm text-foreground/90">
            {lengthLabel && (
              <li className="flex items-center gap-3">
                <PlaySquare className="size-4 shrink-0 text-muted-foreground" />
                {t("hoursOnDemandVideo", { length: lengthLabel })}
              </li>
            )}
            {videoCount > 0 && !lengthLabel && (
              <li className="flex items-center gap-3">
                <PlaySquare className="size-4 shrink-0 text-muted-foreground" />
                {t("videoLectures", { count: videoCount })}
              </li>
            )}
            {resourceCount > 0 && (
              <li className="flex items-center gap-3">
                <FileText className="size-4 shrink-0 text-muted-foreground" />
                {t("practiceResources", { count: resourceCount })}
              </li>
            )}
            <li className="flex items-center gap-3">
              <MonitorSmartphone className="size-4 shrink-0 text-muted-foreground" />
              {t("accessOnMobile")}
            </li>
            <li className="flex items-center gap-3">
              <InfinityIcon className="size-4 shrink-0 text-muted-foreground" />
              {t("fullLifetimeAccess")}
            </li>
          </ul>
        </div>
      </div>
    </div>
  );

  return (
    // Outer grid: [gutter | 1340px content | gutter] × [hero row | body row].
    // The dark band is a full-width item in the hero row, so its height
    // follows the hero text; the content column is a row-subgrid so the
    // sticky buy card can start inside the band and run down the body.
    <div className="grid grid-cols-[minmax(1rem,1fr)_minmax(0,1340px)_minmax(1rem,1fr)] grid-rows-[auto_1fr] sm:grid-cols-[minmax(1.5rem,1fr)_minmax(0,1340px)_minmax(1.5rem,1fr)]">
      <div aria-hidden className="col-span-full row-start-1 bg-ink" />

      <div className="col-start-2 row-span-2 row-start-1 grid grid-rows-subgrid gap-x-12 lg:grid-cols-[minmax(0,1fr)_340px]">
        {/* Hero text — white on the band. */}
        <div className="col-start-1 row-start-1 max-w-[700px] py-8 text-ink-foreground">
          <nav className="flex items-center gap-1 text-sm font-bold text-[#cec0fc]">
            <Link href="/courses/catalog" className="hover:underline">
              {t("allCourses")}
            </Link>
            <ChevronRight className="size-3.5" aria-hidden />
            <Link href={`/teachers/${course.teacher.slug}`} className="hover:underline">
              {course.teacher.displayName}
            </Link>
          </nav>

          <div className="mt-4 lg:hidden">{cover}</div>

          <h1 className="mt-4 font-display text-[1.75rem] leading-tight sm:text-[2rem]">
            {course.title}
          </h1>
          {course.subtitle && <p className="mt-2 text-lg text-white/90">{course.subtitle}</p>}

          <div className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
            {course.reviewCount > 0 ? (
              <>
                <span className="font-bold text-[#f69c08]">{course.rating.toFixed(1)}</span>
                <Stars value={course.rating} className="[&_svg]:size-3.5" />
                <a href="#reviews" className="text-[#cec0fc] underline underline-offset-2">
                  {t("ratingCount", { count: course.reviewCount })}
                </a>
              </>
            ) : (
              <span className="text-white/70">{t("noRatingsShort")}</span>
            )}
          </div>

          <p className="mt-2 text-sm">
            {t("createdBy")}{" "}
            <Link
              href={`/teachers/${course.teacher.slug}`}
              className="font-bold text-[#cec0fc] underline underline-offset-2"
            >
              {course.teacher.displayName}
            </Link>
          </p>

          <p className="mt-2 inline-flex items-center gap-1.5 text-sm text-white/80">
            <Globe className="size-4" aria-hidden />
            {t("selfPaced")}
          </p>
        </div>

        {/* Body. */}
        <div className="col-start-1 row-start-2 max-w-[700px] py-8">
          {/* Buy card on phones sits right under the hero. */}
          <div className="mb-8 lg:hidden">{buyCard}</div>

            <section>
              <h2 className="font-display text-2xl">{t("courseContent")}</h2>
              <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-sm">
                <p className="text-foreground/90">
                  {[
                    t("sections", { count: course.sections.length }),
                    t("lectures", { count: totalItems }),
                    lengthLabel ? t("totalLength", { length: lengthLabel }) : null,
                  ]
                    .filter(Boolean)
                    .join(" • ")}
                </p>
                <button
                  type="button"
                  onClick={() => setExpandedAll((v) => !v)}
                  className="font-bold text-link hover:underline"
                >
                  {expandedAll ? t("collapseAll") : t("expandAll")}
                </button>
              </div>

              <div className="mt-4 border border-border">
                {course.sections.map((section, i) => (
                  <CurriculumSection
                    key={`${section.id}-${expandedAll}`}
                    title={section.title}
                    defaultOpen={expandedAll || i === 0}
                    meta={[
                      t("lectures", { count: section.items.length }),
                      (() => {
                        const secs = section.items.reduce((n, it) => n + it.durationSeconds, 0);
                        return formatLectureLength(secs);
                      })(),
                    ]
                      .filter(Boolean)
                      .join(" • ")}
                  >
                    {section.items.map((item) => {
                      const itemLength = formatLectureLength(item.durationSeconds);
                      return (
                        <li
                          key={item.id}
                          className="flex items-center gap-3 px-4 py-2 text-sm text-foreground/90"
                        >
                          {item.kind === "video" ? (
                            <PlaySquare className="size-4 shrink-0 text-muted-foreground" />
                          ) : (
                            <FileText className="size-4 shrink-0 text-muted-foreground" />
                          )}
                          <span className="min-w-0 flex-1 truncate">
                            {item.title || (item.kind === "video" ? t("video") : t("resource"))}
                          </span>
                          {item.isPreview && (
                            <button
                              type="button"
                              onClick={() => {
                                setPreviewItemId(item.id);
                                window.scrollTo({ top: 0, behavior: "smooth" });
                              }}
                              className="shrink-0 font-bold text-link underline underline-offset-2"
                            >
                              {t("previewLesson")}
                            </button>
                          )}
                          {itemLength && (
                            <span className="shrink-0 text-xs tabular-nums text-muted-foreground">
                              {itemLength}
                            </span>
                          )}
                        </li>
                      );
                    })}
                    {section.items.length === 0 && (
                      <li className="px-4 py-2 text-sm text-muted-foreground">{t("nothingHereYet")}</li>
                    )}
                  </CurriculumSection>
                ))}
                {course.sections.length === 0 && (
                  <p className="px-4 py-3 text-sm text-muted-foreground">
                    {t("curriculumNotPublished")}
                  </p>
                )}
              </div>
            </section>

            {course.description && (
              <section className="mt-10">
                <h2 className="font-display text-2xl">{t("descriptionHeading")}</h2>
                <div
                  className={cn(
                    "relative mt-3 space-y-4 text-sm leading-relaxed text-foreground/90",
                    !descriptionOpen && "max-h-56 overflow-hidden",
                  )}
                >
                  {course.description.split("\n\n").map((para, i) => (
                    <p key={i}>{para}</p>
                  ))}
                  {!descriptionOpen && course.description.length > 600 && (
                    <div className="pointer-events-none absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-background to-transparent" />
                  )}
                </div>
                {course.description.length > 600 && (
                  <button
                    type="button"
                    onClick={() => setDescriptionOpen((v) => !v)}
                    className="mt-2 inline-flex items-center gap-1 text-sm font-bold text-link hover:underline"
                  >
                    {descriptionOpen ? t("showLess") : t("showMore")}
                    <ChevronDown className={cn("size-4", descriptionOpen && "rotate-180")} />
                  </button>
                )}
              </section>
            )}

            <section className="mt-10">
              <h2 className="font-display text-2xl">{t("instructor")}</h2>
              <Link
                href={`/teachers/${course.teacher.slug}`}
                className="mt-3 inline-block text-lg font-bold text-link underline underline-offset-2"
              >
                {course.teacher.displayName}
              </Link>
              <p className="mt-1 text-sm text-muted-foreground">{t("instructorBlurb")}</p>
            </section>

            {/* Phase D2. `canReview` is the enrolled buyer only — the owner and
                a browsing visitor get the list and the histogram, not the form. */}
            <div id="reviews" className="mt-10">
              <CourseReviews
                courseId={course.id}
                rating={course.rating}
                reviewCount={course.reviewCount}
                canReview={enrolled && !course.isOwner}
                myReview={course.myReview}
              />
            </div>
        </div>

        {/* Sticky buy card, starting inside the dark band on desktop. */}
        <aside className="hidden lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:block lg:pt-8">
          <div className="sticky top-[88px] mb-8">{buyCard}</div>
        </aside>
      </div>
    </div>
  );
}

function CurriculumSection({
  title,
  meta,
  defaultOpen,
  children,
}: {
  title: string;
  meta: string;
  defaultOpen: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-border last:border-b-0">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center gap-3 bg-muted px-4 py-3 text-left"
      >
        <ChevronDown className={cn("size-4 shrink-0 transition-transform", open && "rotate-180")} />
        <span className="min-w-0 flex-1 truncate text-base font-bold text-foreground">{title}</span>
        <span className="shrink-0 text-xs text-muted-foreground">{meta}</span>
      </button>
      {open && <ul className="py-1">{children}</ul>}
    </div>
  );
}
