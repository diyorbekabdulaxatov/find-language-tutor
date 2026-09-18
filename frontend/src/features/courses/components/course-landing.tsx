"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import {
  ArrowLeft,
  CheckCircle2,
  Clock,
  FileText,
  Film,
  GraduationCap,
  Layers,
  PlayCircle,
} from "lucide-react";
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
      <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-10">
        <div className="grid gap-6 lg:grid-cols-[1fr_340px]">
          <div className="h-80 animate-pulse rounded-2xl bg-muted lg:col-start-1" />
          <div className="h-64 animate-pulse rounded-2xl bg-muted lg:col-start-2" />
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
  const totalItems = course.sections.reduce((n, s) => n + s.items.length, 0);
  const length = courseLengthParts(course.totalDurationSeconds);
  // The first free lecture in curriculum order — what the hero's play button
  // starts with. null when the teacher has not marked any.
  const firstPreview =
    course.sections.flatMap((s) => s.items).find((i) => i.isPreview) ?? null;
  const enrolled = course.isEnrolled || justEnrolled !== null;

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-10">
      <Link
        href="/courses/catalog"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        {t("allCourses")}
      </Link>

      <div className="mt-5 grid gap-6 lg:grid-cols-[1fr_340px]">
        {/* Header card */}
        <div className="overflow-hidden rounded-2xl bg-card ring-1 ring-border shadow-card lg:col-start-1 lg:row-start-1">
          <div className="relative aspect-[16/7] bg-muted">
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
              <img
                src={courseCoverUrl(course.id)}
                alt=""
                className="size-full object-cover"
              />
            ) : (
              <div className="grid size-full place-items-center text-muted-foreground">
                <Film className="size-10" />
              </div>
            )}

            {/* The cover doubles as the play surface when a free sample
                exists — the "try before you buy" affordance shoppers look
                for before they read anything else. */}
            {!previewItemId && firstPreview && (
              <button
                type="button"
                onClick={() => setPreviewItemId(firstPreview.id)}
                className="absolute inset-0 grid place-items-center bg-foreground/25 transition-colors hover:bg-foreground/35 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
              >
                <span className="inline-flex items-center gap-2 rounded-full bg-background/95 px-4 py-2.5 text-sm font-semibold text-foreground shadow-lift backdrop-blur">
                  <PlayCircle className="size-5" />
                  {t("watchFreePreview")}
                </span>
              </button>
            )}
          </div>

          <div className="p-5 sm:p-6">
            <h1 className="font-display text-3xl tracking-tight sm:text-[2rem]">
              {course.title}
            </h1>
            {course.subtitle && (
              <p className="mt-1.5 text-muted-foreground">{course.subtitle}</p>
            )}

            {course.reviewCount > 0 && (
              <p className="mt-2.5 inline-flex items-center gap-2 text-sm">
                <span className="font-semibold text-foreground">
                  {course.rating.toFixed(1)}
                </span>
                <Stars value={course.rating} />
                <span className="text-muted-foreground">
                  {t("ratingCount", { count: course.reviewCount })}
                </span>
              </p>
            )}

            <Link
              href={`/teachers/${course.teacher.slug}`}
              className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-foreground hover:text-primary hover:underline"
            >
              <GraduationCap className="size-4 text-muted-foreground" />
              {course.teacher.displayName}
            </Link>

            <p className="mt-2 inline-flex flex-wrap items-center gap-x-1.5 gap-y-1 text-sm text-muted-foreground">
              <Layers className="size-4" />
              {t("sections", { count: course.sections.length })} · {t("lessons", { count: totalItems })}
              {length && (
                <>
                  <span aria-hidden>·</span>
                  <Clock className="size-4" />
                  {length.hours > 0
                    ? t("courseLengthHm", { hours: length.hours, minutes: length.minutes })
                    : t("courseLengthM", { minutes: length.minutes })}
                </>
              )}
            </p>
          </div>
        </div>

        {/* CTA panel — sticky on desktop */}
        <aside className="lg:sticky lg:top-20 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:self-start">
          <div className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-card">
            <div className="flex items-end gap-1.5">
              <span className="font-display text-3xl text-foreground">
                {free ? t("free") : formatMoney(course.price, locale)}
              </span>
            </div>

            <div className="mt-5">
              {course.isOwner ? (
                <Button asChild size="lg" className="w-full">
                  <Link href={`/courses/${course.id}/edit`}>{t("editCourse")}</Link>
                </Button>
              ) : enrolled ? (
                <div className="flex flex-col gap-3">
                  {justEnrolled && (
                    <p className="inline-flex items-center gap-2 rounded-lg bg-mint/12 px-3 py-2 text-sm font-medium text-mint">
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
              <p className="mt-5 rounded-xl bg-primary/8 p-3 text-xs text-muted-foreground">
                {t("lifetimeAccess")}
              </p>
            )}
          </div>
        </aside>

        {/* Long-form content */}
        <div className="space-y-6 lg:col-start-1 lg:row-start-2">
          {course.description && (
            <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
              <h2 className="font-display text-xl">{t("aboutCourse")}</h2>
              <div className="mt-3 max-w-[64ch] space-y-4 text-sm leading-relaxed text-foreground/90">
                {course.description.split("\n\n").map((para, i) => (
                  <p key={i}>{para}</p>
                ))}
              </div>
            </section>
          )}

          <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
            <h2 className="font-display text-xl">{t("curriculum")}</h2>
            <ul className="mt-4 flex flex-col gap-4">
              {course.sections.map((section) => (
                <li key={section.id}>
                  <p className="font-medium text-foreground">{section.title}</p>
                  <ul className="mt-2 flex flex-col gap-1.5 border-l border-border pl-4">
                    {section.items.map((item) => {
                      const itemLength = formatLectureLength(item.durationSeconds);
                      return (
                        <li
                          key={item.id}
                          className="flex items-center gap-2 text-sm text-muted-foreground"
                        >
                          {item.kind === "video" ? (
                            <Film className="size-3.5 shrink-0" />
                          ) : (
                            <FileText className="size-3.5 shrink-0" />
                          )}
                          <span className="min-w-0 flex-1 truncate">
                            {item.title || (item.kind === "video" ? t("video") : t("resource"))}
                          </span>

                          {item.isPreview && (
                            <button
                              type="button"
                              onClick={() => setPreviewItemId(item.id)}
                              className="inline-flex shrink-0 items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-semibold text-primary transition-colors hover:bg-primary/20"
                            >
                              <PlayCircle className="size-3.5" />
                              {t("previewLesson")}
                            </button>
                          )}

                          {itemLength && (
                            <span className="shrink-0 font-mono text-xs tabular-nums">
                              {itemLength}
                            </span>
                          )}
                        </li>
                      );
                    })}
                    {section.items.length === 0 && (
                      <li className="text-sm text-muted-foreground/70">{t("nothingHereYet")}</li>
                    )}
                  </ul>
                </li>
              ))}
              {course.sections.length === 0 && (
                <li className="text-sm text-muted-foreground">
                  {t("curriculumNotPublished")}
                </li>
              )}
            </ul>
          </section>

          {/* Phase D2. `canReview` is the enrolled buyer only — the owner and
              a browsing visitor get the list and the histogram, not the form. */}
          <CourseReviews
            courseId={course.id}
            rating={course.rating}
            reviewCount={course.reviewCount}
            canReview={enrolled && !course.isOwner}
            myReview={course.myReview}
          />
        </div>
      </div>
    </div>
  );
}
