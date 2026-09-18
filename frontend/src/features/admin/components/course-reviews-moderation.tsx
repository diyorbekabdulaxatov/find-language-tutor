"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { intlLocale } from "@/lib/i18n";
import {
  AdminError,
  hideCourseReview,
  listAdminCourseReviews,
  unhideCourseReview,
  type AdminCourseReview,
  type ReviewVisibility,
} from "@/features/admin/api";
import { Stars } from "@/features/reviews/components/star-rating";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

const VISIBILITY: { value: ReviewVisibility; label: "visAll" | "visVisible" | "visHidden" }[] = [
  { value: "all", label: "visAll" },
  { value: "visible", label: "visVisible" },
  { value: "hidden", label: "visHidden" },
];

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

/**
 * The course-review moderation queue. Deliberately the same shape as
 * `ReviewsModeration` (lesson reviews) — same filters, same card, same
 * hide/restore verbs — because it is the same job on a different subject, and
 * it is guarded by the same `reviews.moderate` permission.
 *
 * Unlike the lesson queue there is no permanent delete: a course review's row
 * is also the one-review-per-buyer gate, so removing it would silently let
 * the author post a replacement. Hiding is the takedown.
 */
export function CourseReviewsModeration() {
  const [visibility, setVisibility] = useState<ReviewVisibility>("all");
  const [lowOnly, setLowOnly] = useState(false);
  const [page, setPage] = useState(1);

  const [rows, setRows] = useState<AdminCourseReview[]>([]);
  const [total, setTotal] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [reloadKey, setReloadKey] = useState(0);
  const t = useTranslations("admin");
  const locale = useLocale();

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const res = await listAdminCourseReviews({
          visibility,
          maxRating: lowOnly ? 3 : undefined,
          page,
        });
        if (!alive) return;
        setRows(res.items);
        setTotal(res.total);
        setState("ready");
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [visibility, lowOnly, page, reloadKey]);

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const reload = () => setReloadKey((k) => k + 1);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {VISIBILITY.map((f) => (
          <button
            key={f.value}
            onClick={() => {
              setPage(1);
              setVisibility(f.value);
            }}
            className={cn(
              "rounded-lg border px-3 py-1.5 text-sm transition-colors",
              visibility === f.value
                ? "border-primary bg-accent"
                : "border-border hover:bg-muted",
            )}
          >
            {t(f.label)}
          </button>
        ))}

        <label className="ml-1 flex items-center gap-2 text-sm text-muted-foreground">
          <input
            type="checkbox"
            checked={lowOnly}
            onChange={(e) => {
              setPage(1);
              setLowOnly(e.target.checked);
            }}
            className="accent-primary"
          />
          {t("lowRated")}
        </label>
      </div>

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoadReviews")}
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : rows.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
          {t("noReviewsMatch")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {rows.map((r) => (
            <CourseReviewCard key={r.id} review={r} onChanged={reload} locale={locale} />
          ))}
        </ul>
      )}

      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">
            {t("pageOf", { total: total.toLocaleString(locale), page, last: lastPage })}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              {t("previous")}
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= lastPage}
              onClick={() => setPage((p) => p + 1)}
            >
              {t("next")}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

function CourseReviewCard({
  review: r,
  onChanged,
  locale,
}: {
  review: AdminCourseReview;
  onChanged: () => void;
  locale: string;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const t = useTranslations("admin");

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onChanged();
    } catch (err) {
      setError(err instanceof AdminError ? err.message : t("somethingWrong"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <li
      className={cn(
        "rounded-2xl border border-border bg-card p-4",
        r.hidden && "opacity-70",
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Stars value={r.rating} />
        <Link
          href={`/courses/catalog/${r.courseId}`}
          className="text-sm font-medium text-foreground hover:text-primary hover:underline"
        >
          {r.courseTitle}
        </Link>
        <span className="text-sm text-muted-foreground">· {r.studentName}</span>
        <span className="text-xs text-muted-foreground">{fmt(r.createdAt, locale)}</span>
        {r.hidden && (
          <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {t("hidden")}
          </span>
        )}
      </div>

      {r.comment && (
        <p className="mt-2 max-w-[70ch] text-sm leading-relaxed text-foreground/90">
          {r.comment}
        </p>
      )}

      {error && <p className="mt-2 text-sm text-destructive">{error}</p>}

      <div className="mt-3 flex gap-2">
        {r.hidden ? (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => void run(() => unhideCourseReview(r.id))}
          >
            {t("restore")}
          </Button>
        ) : (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => void run(() => hideCourseReview(r.id))}
          >
            {t("hide")}
          </Button>
        )}
      </div>
    </li>
  );
}
