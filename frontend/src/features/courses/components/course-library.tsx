"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { GraduationCap, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/format";
import {
  CourseError,
  listCourses,
  setCourseArchived,
} from "@/features/courses/api";
import type { Course, CourseStatus } from "@/features/courses/types";

type StatusFilter = CourseStatus | "all";

export function CourseLibrary() {
  const [status, setStatus] = useState<StatusFilter>("all");
  const [showArchived, setShowArchived] = useState(false);

  const [items, setItems] = useState<Course[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error" | "no-teacher">("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const res = await listCourses({
          status: status === "all" ? undefined : status,
          archived: showArchived,
        });
        if (!alive) return;
        setItems(res.courses);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setState(err instanceof CourseError && err.code === "no_teacher_profile" ? "no-teacher" : "error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [status, showArchived, reloadKey]);

  const reload = () => setReloadKey((k) => k + 1);

  if (state === "no-teacher") {
    return (
      <div className="rounded-2xl border border-border bg-card px-6 py-12 text-center">
        <p className="text-sm text-muted-foreground">
          Courses are part of your teaching toolkit — create a teacher profile
          first.
        </p>
        <Button asChild className="mt-4">
          <Link href="/dashboard">Go to the dashboard</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-display text-3xl">My courses</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Build self-paced video courses from your lessons and resources.
          </p>
        </div>
        <Button asChild>
          <Link href="/courses/new">
            <Plus className="size-4" />
            New course
          </Link>
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {(["all", "draft", "published"] as StatusFilter[]).map((s) => (
          <Chip key={s} active={status === s} onClick={() => setStatus(s)}>
            {s === "all" ? "Any status" : s}
          </Chip>
        ))}
        <label className="ml-1 flex items-center gap-1.5 text-sm text-muted-foreground">
          <input
            type="checkbox"
            checked={showArchived}
            onChange={(e) => setShowArchived(e.target.checked)}
            className="accent-primary"
          />
          Archived
        </label>
      </div>

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load your courses.
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : items.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-12 text-center text-sm text-muted-foreground">
          Nothing here yet.
        </p>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {items.map((c) => (
            <CourseCard key={c.id} course={c} onChanged={reload} />
          ))}
        </ul>
      )}
    </div>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "rounded-lg border px-2.5 py-1 text-xs font-medium capitalize transition-colors",
        active ? "border-primary bg-accent" : "border-border hover:bg-muted",
      )}
    >
      {children}
    </button>
  );
}

function CourseCard({ course: c, onChanged }: { course: Course; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    try {
      await fn();
      onChanged();
    } catch {
      setBusy(false);
    }
  }

  return (
    <li className="flex flex-col gap-3 rounded-2xl border border-border bg-card p-4">
      <div className="flex items-start gap-3">
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
          <GraduationCap className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <Link
            href={`/courses/${c.id}/edit`}
            className="block truncate font-medium hover:text-primary hover:underline"
          >
            {c.title}
          </Link>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">
            {c.subtitle || formatMoney(c.price)}
          </p>
        </div>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold",
            c.archived
              ? "bg-muted text-muted-foreground"
              : c.status === "published"
                ? "bg-primary/15 text-primary"
                : "bg-star/15 text-star",
          )}
        >
          {c.archived ? "Archived" : c.status}
        </span>
      </div>

      <div className="flex flex-wrap gap-2 border-t border-border pt-3">
        <Button asChild size="sm" variant="outline">
          <Link href={`/courses/${c.id}/edit`}>Edit</Link>
        </Button>
        <span className="ml-auto self-center text-xs text-muted-foreground">
          {formatMoney(c.price)}
        </span>
        {!c.archived && (
          <Button
            size="sm"
            variant="ghost"
            className="text-muted-foreground"
            disabled={busy}
            onClick={() => run(() => setCourseArchived(c.id, true))}
          >
            Archive
          </Button>
        )}
        {c.archived && (
          <Button
            size="sm"
            variant="ghost"
            className="text-muted-foreground"
            disabled={busy}
            onClick={() => run(() => setCourseArchived(c.id, false))}
          >
            Restore
          </Button>
        )}
      </div>
    </li>
  );
}
