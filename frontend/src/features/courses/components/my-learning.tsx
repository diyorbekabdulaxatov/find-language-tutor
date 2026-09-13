"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Film } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { courseCoverUrl, listEnrollments } from "@/features/courses/api";
import type { CourseEnrollment } from "@/features/courses/types";

export function MyLearning() {
  const [enrollments, setEnrollments] = useState<CourseEnrollment[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

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
    <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 lg:py-14">
      <header className="max-w-2xl">
        <h1 className="font-display text-3xl tracking-tight sm:text-4xl">My learning</h1>
        <p className="mt-3 text-muted-foreground">
          Courses you&rsquo;ve enrolled in — pick up where you left off.
        </p>
      </header>

      <div className="mt-8">
        {state === "loading" && (
          <div className="space-y-3">
            {[0, 1, 2].map((i) => (
              <div key={i} className="h-24 animate-pulse rounded-2xl bg-muted" />
            ))}
          </div>
        )}

        {state === "error" && (
          <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            Could not load your courses. Please try again.
          </p>
        )}

        {state === "ready" && enrollments.length === 0 && (
          <div className="rounded-2xl border border-dashed border-border bg-card p-12 text-center">
            <p className="font-display text-xl">Nothing here yet</p>
            <p className="mt-2 text-sm text-muted-foreground">
              Enroll in a course to see it in your learning list.
            </p>
            <Link
              href="/courses/catalog"
              className="mt-4 inline-flex h-10 items-center rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
            >
              Browse courses
            </Link>
          </div>
        )}

        {state === "ready" && enrollments.length > 0 && (
          <ul className="flex flex-col gap-3">
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

  return (
    <li className="flex flex-col gap-4 rounded-2xl bg-card p-4 ring-1 ring-border shadow-soft sm:flex-row sm:items-center">
      <div className="relative aspect-[16/10] w-full shrink-0 overflow-hidden rounded-xl bg-muted sm:w-40">
        {course.coverAssetId ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={courseCoverUrl(course.id)} alt="" className="size-full object-cover" />
        ) : (
          <div className="grid size-full place-items-center text-muted-foreground">
            <Film className="size-6" />
          </div>
        )}
      </div>

      <div className="min-w-0 flex-1">
        <Link href={`/learn/${course.id}`} className="font-display text-lg hover:text-primary hover:underline">
          {course.title}
        </Link>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {enrollment.source === "free" ? "Free" : formatMoney(enrollment.amountPaid)} ·{" "}
          enrolled {new Date(enrollment.createdAt).toLocaleDateString("en-US", {
            month: "short",
            day: "numeric",
            year: "numeric",
          })}
        </p>

        <div className="mt-3 flex items-center gap-2.5">
          <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
            <div
              className={complete ? "h-full rounded-full bg-mint" : "h-full rounded-full bg-primary"}
              style={{ width: `${Math.min(100, Math.max(0, enrollment.progressPercent))}%` }}
            />
          </div>
          <span className="shrink-0 text-xs font-medium text-muted-foreground">
            {enrollment.progressPercent}%
          </span>
        </div>
      </div>

      <Link
        href={`/learn/${course.id}`}
        className="inline-flex h-10 shrink-0 items-center justify-center rounded-lg bg-primary px-4 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
      >
        {complete ? "Review" : "Continue"}
      </Link>
    </li>
  );
}
