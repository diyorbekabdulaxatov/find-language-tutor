"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Film, PlayCircle } from "lucide-react";
import { intlLocale } from "@/lib/i18n";
import { courseCoverUrl, listEnrollments } from "@/features/courses/api";
import type { CourseEnrollment } from "@/features/courses/types";

export function MyLearning() {
  const [enrollments, setEnrollments] = useState<CourseEnrollment[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const t = useTranslations("courses");

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const list = await listEnrollments();
        if (alive) {
          setEnrollments(list);
          setState("ready");
        }
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div>
      {/* Udemy's My learning: a dark title band with the tab row on its bottom edge. */}
      <div className="bg-ink text-ink-foreground">
        <div className="mx-auto max-w-[1340px] px-4 pt-10 sm:px-6">
          <h1 className="font-display text-3xl sm:text-[2.5rem]">{t("learningTitle")}</h1>
          <p className="mt-2 text-white/80">{t("learningIntro")}</p>
          <div className="mt-6 flex gap-6 text-base font-bold">
            <span className="-mb-px border-b-4 border-white pb-2">{t("allCourses")}</span>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6">
        {state === "loading" && (
          <div className="grid gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {[0, 1, 2, 3].map((i) => (
              <div key={i} className="aspect-[4/3] animate-pulse bg-muted" />
            ))}
          </div>
        )}

        {state === "error" && (
          <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {t("couldNotLoadCourses")}
          </p>
        )}

        {state === "ready" && enrollments.length === 0 && (
          <div className="border border-border px-6 py-16 text-center">
            <p className="text-xl font-bold">{t("nothingYetTitle")}</p>
            <p className="mt-2 text-sm text-muted-foreground">{t("nothingYetBody")}</p>
            <Link
              href="/courses/catalog"
              className="mt-4 inline-flex h-12 items-center rounded-md bg-primary px-4 text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
            >
              {t("browseCourses")}
            </Link>
          </div>
        )}

        {state === "ready" && enrollments.length > 0 && (
          <ul className="grid gap-6 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {enrollments.map((e) => (
              <EnrollmentRow key={e.id} enrollment={e} />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function EnrollmentRow({ enrollment }: { enrollment: CourseEnrollment }) {
  const { course } = enrollment;
  const complete = enrollment.progressPercent >= 100;
  const t = useTranslations("courses");
  const locale = useLocale();
  const pct = Math.min(100, Math.max(0, enrollment.progressPercent));

  return (
    <li>
      <Link href={`/learn/${course.id}`} className="group block outline-none focus-visible:ring-3 focus-visible:ring-ring/40">
        <div className="relative aspect-[16/9] overflow-hidden border border-border bg-muted">
          {course.coverAssetId ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={courseCoverUrl(course.id)} alt="" className="size-full object-cover" />
          ) : (
            <div className="grid size-full place-items-center text-muted-foreground">
              <Film className="size-8" />
            </div>
          )}
          <span className="absolute inset-0 grid place-items-center bg-black/0 text-white opacity-0 transition-opacity group-hover:bg-black/40 group-hover:opacity-100">
            <span className="grid size-14 place-items-center rounded-full bg-white text-ink">
              <PlayCircle className="size-7" />
            </span>
          </span>
        </div>
        <p className="mt-2 line-clamp-2 text-base leading-tight font-bold text-foreground group-hover:text-link">
          {course.title}
        </p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {t("enrolledOn", {
            date: new Date(enrollment.createdAt).toLocaleDateString(intlLocale(locale), {
              month: "short",
              day: "numeric",
              year: "numeric",
            }),
          })}
        </p>
        <div className="mt-2 h-1 w-full bg-border">
          <div className={complete ? "h-full bg-mint" : "h-full bg-primary"} style={{ width: `${pct}%` }} />
        </div>
        <p className="mt-1 text-xs text-muted-foreground">
          {pct === 0 ? t("startCourse") : t("percentComplete", { pct })}
        </p>
      </Link>
    </li>
  );
}
