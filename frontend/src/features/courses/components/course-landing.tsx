"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, CheckCircle2, FileText, Film, GraduationCap, Layers } from "lucide-react";
import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/format";
import { useAuth } from "@/features/auth/auth-context";
import { refreshSession } from "@/features/auth/browser-client";
import { courseCoverUrl, getMyCourseCatalogDetail } from "@/features/courses/api";
import type { CourseCatalogDetail, CourseEnrollment } from "@/features/courses/types";
import { PurchaseCoursePanel } from "./purchase-course-panel";

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
        <p className="font-display text-2xl">Course not found</p>
        <p className="mt-2 text-sm text-muted-foreground">
          It may have been unpublished, or the link is wrong.
        </p>
        <Button asChild className="mt-6">
          <Link href="/courses/catalog">Browse courses</Link>
        </Button>
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load this course. Please try again.
        </p>
      </div>
    );
  }

  const free = course.price.amountMinor === 0;
  const totalItems = course.sections.reduce((n, s) => n + s.items.length, 0);
  const enrolled = course.isEnrolled || justEnrolled !== null;

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-10">
      <Link
        href="/courses/catalog"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        All courses
      </Link>

      <div className="mt-5 grid gap-6 lg:grid-cols-[1fr_340px]">
        {/* Header card */}
        <div className="overflow-hidden rounded-2xl bg-card ring-1 ring-border shadow-card lg:col-start-1 lg:row-start-1">
          <div className="relative aspect-[16/7] bg-muted">
            {course.coverAssetId ? (
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
          </div>

          <div className="p-5 sm:p-6">
            <h1 className="font-display text-3xl tracking-tight sm:text-[2rem]">
              {course.title}
            </h1>
            {course.subtitle && (
              <p className="mt-1.5 text-muted-foreground">{course.subtitle}</p>
            )}

            <Link
              href={`/teachers/${course.teacher.slug}`}
              className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-foreground hover:text-primary hover:underline"
            >
              <GraduationCap className="size-4 text-muted-foreground" />
              {course.teacher.displayName}
            </Link>

            <p className="mt-2 inline-flex items-center gap-1.5 text-sm text-muted-foreground">
              <Layers className="size-4" />
              {course.sections.length} {course.sections.length === 1 ? "section" : "sections"} ·{" "}
              {totalItems} {totalItems === 1 ? "lesson" : "lessons"}
            </p>
          </div>
        </div>

        {/* CTA panel — sticky on desktop */}
        <aside className="lg:sticky lg:top-20 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:self-start">
          <div className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-card">
            <div className="flex items-end gap-1.5">
              <span className="font-display text-3xl text-foreground">
                {free ? "Free" : formatMoney(course.price)}
              </span>
            </div>

            <div className="mt-5">
              {course.isOwner ? (
                <Button asChild size="lg" className="w-full">
                  <Link href={`/courses/${course.id}/edit`}>Edit course</Link>
                </Button>
              ) : enrolled ? (
                <div className="flex flex-col gap-3">
                  {justEnrolled && (
                    <p className="inline-flex items-center gap-2 rounded-lg bg-mint/12 px-3 py-2 text-sm font-medium text-mint">
                      <CheckCircle2 className="size-4" />
                      You&rsquo;re enrolled
                    </p>
                  )}
                  <Button asChild size="lg" className="w-full">
                    <Link href={`/learn/${course.id}`}>Continue learning</Link>
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
                Watch on your own schedule — lifetime access once you enroll.
              </p>
            )}
          </div>
        </aside>

        {/* Long-form content */}
        <div className="space-y-6 lg:col-start-1 lg:row-start-2">
          {course.description && (
            <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
              <h2 className="font-display text-xl">About this course</h2>
              <div className="mt-3 max-w-[64ch] space-y-4 text-sm leading-relaxed text-foreground/90">
                {course.description.split("\n\n").map((para, i) => (
                  <p key={i}>{para}</p>
                ))}
              </div>
            </section>
          )}

          <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
            <h2 className="font-display text-xl">Curriculum</h2>
            <ul className="mt-4 flex flex-col gap-4">
              {course.sections.map((section) => (
                <li key={section.id}>
                  <p className="font-medium text-foreground">{section.title}</p>
                  <ul className="mt-2 flex flex-col gap-1.5 border-l border-border pl-4">
                    {section.items.map((item) => (
                      <li
                        key={item.id}
                        className="flex items-center gap-2 text-sm text-muted-foreground"
                      >
                        {item.kind === "video" ? (
                          <Film className="size-3.5 shrink-0" />
                        ) : (
                          <FileText className="size-3.5 shrink-0" />
                        )}
                        {item.title || (item.kind === "video" ? "Video" : "Resource")}
                      </li>
                    ))}
                    {section.items.length === 0 && (
                      <li className="text-sm text-muted-foreground/70">Nothing here yet.</li>
                    )}
                  </ul>
                </li>
              ))}
              {course.sections.length === 0 && (
                <li className="text-sm text-muted-foreground">
                  This course&rsquo;s curriculum isn&rsquo;t published yet.
                </li>
              )}
            </ul>
          </section>
        </div>
      </div>
    </div>
  );
}
