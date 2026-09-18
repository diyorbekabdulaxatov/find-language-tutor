"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { intlLocale } from "@/lib/i18n";
import { Stars, StarInput } from "@/features/reviews/components/star-rating";
import {
  CourseError,
  createCourseReview,
  listCourseReviews,
  updateCourseReview,
} from "@/features/courses/api";
import type { CourseReview, CourseReviewPage } from "@/features/courses/types";

function formatDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

/**
 * The reviews block on a course landing page: the aggregate, the star
 * histogram, the list, and — for a buyer — the form to write or revise their
 * own review.
 *
 * `myReview` comes down with the landing page itself rather than being probed
 * for, so the form opens in the right mode on first paint: a buyer who has
 * already reviewed sees "edit yours", never a "write one" button that would
 * come back 409.
 */
export function CourseReviews({
  courseId,
  rating,
  reviewCount,
  canReview,
  myReview: initialMyReview,
}: {
  courseId: string;
  rating: number;
  reviewCount: number;
  /** True only for an enrolled buyer — the owner and non-buyers can't. */
  canReview: boolean;
  myReview: CourseReview | null;
}) {
  const [page, setPage] = useState<CourseReviewPage | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [myReview, setMyReview] = useState(initialMyReview);
  const [editing, setEditing] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const t = useTranslations("courses");
  const locale = useLocale();

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const res = await listCourseReviews(courseId);
        if (!alive) return;
        setPage(res);
        setState("ready");
      } catch {
        if (!alive) return;
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [courseId, reloadKey]);

  // A fresh write / revision is spliced into the list so the author sees
  // their own words immediately, then the page is refetched so the histogram
  // and aggregate catch up (they are computed server-side over every visible
  // review, not just this page).
  function onSaved(saved: CourseReview) {
    setMyReview(saved);
    setEditing(false);
    setPage((prev) => {
      if (!prev) return prev;
      const others = prev.reviews.filter((r) => r.id !== saved.id);
      const isNew = others.length === prev.reviews.length;
      return {
        ...prev,
        reviews: [saved, ...others],
        total: isNew ? prev.total + 1 : prev.total,
      };
    });
    setReloadKey((k) => k + 1);
  }

  // The landing page's `rating` / `reviewCount` were fetched once with the
  // course; once the review list is in hand, derive the same numbers from its
  // histogram so a just-posted review is reflected without a full reload.
  const shownCount = page ? page.total : reviewCount;
  const shownRating = page
    ? page.total > 0
      ? page.breakdown.reduce((sum, n, i) => sum + n * (i + 1), 0) / page.total
      : 0
    : rating;
  const hasRatings = shownCount > 0;

  return (
    <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
      <h2 className="font-display text-xl">{t("reviewsHeading")}</h2>

      {hasRatings ? (
        <div className="mt-4 flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-8">
          <div className="shrink-0">
            <p className="font-display text-4xl leading-none text-foreground">
              {shownRating.toFixed(1)}
            </p>
            <Stars value={shownRating} className="mt-1.5" />
            <p className="mt-1 text-xs text-muted-foreground">
              {t("ratingCount", { count: shownCount })}
            </p>
          </div>

          {page && (
            <ul className="flex flex-1 flex-col gap-1">
              {[5, 4, 3, 2, 1].map((star) => {
                const n = page.breakdown[star - 1] ?? 0;
                const pct = page.total > 0 ? (n / page.total) * 100 : 0;
                return (
                  <li key={star} className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span className="w-8 shrink-0 tabular-nums">{t("starsShort", { count: star })}</span>
                    <span className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                      <span
                        className="block h-full rounded-full bg-star"
                        style={{ width: `${pct}%` }}
                      />
                    </span>
                    <span className="w-6 shrink-0 text-right tabular-nums">{n}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      ) : (
        <p className="mt-3 text-sm text-muted-foreground">{t("noRatingsYet")}</p>
      )}

      {canReview && (
        <div className="mt-6 border-t border-border pt-5">
          {myReview && !editing ? (
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-foreground">{t("yourReview")}</p>
                <Stars value={myReview.rating} className="mt-1" />
                {myReview.comment && (
                  <p className="mt-1.5 max-w-[60ch] text-sm text-muted-foreground">
                    {myReview.comment}
                  </p>
                )}
              </div>
              <Button type="button" variant="outline" size="sm" onClick={() => setEditing(true)}>
                {t("editReview")}
              </Button>
            </div>
          ) : (
            <ReviewForm
              courseId={courseId}
              existing={myReview}
              onSaved={onSaved}
              onCancel={myReview ? () => setEditing(false) : undefined}
            />
          )}
        </div>
      )}

      <div className="mt-6 border-t border-border pt-5">
        {state === "loading" && <div className="h-16 animate-pulse rounded-lg bg-muted" />}
        {state === "error" && (
          <p className="text-sm text-destructive">{t("couldNotLoadReviews")}</p>
        )}
        {state === "ready" && page && page.reviews.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("noReviewsYet")}</p>
        )}
        {state === "ready" && page && page.reviews.length > 0 && (
          <ul className="flex flex-col gap-5">
            {page.reviews.map((r) => (
              <li key={r.id}>
                <div className="flex items-center gap-2">
                  <Stars value={r.rating} />
                  <span className="text-sm font-medium text-foreground">
                    {r.studentDisplayName}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {formatDate(r.createdAt, locale)}
                  </span>
                </div>
                {r.comment && (
                  <p className="mt-1.5 max-w-[68ch] text-sm leading-relaxed text-foreground/90">
                    {r.comment}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

function ReviewForm({
  courseId,
  existing,
  onSaved,
  onCancel,
}: {
  courseId: string;
  existing: CourseReview | null;
  onSaved: (r: CourseReview) => void;
  onCancel?: () => void;
}) {
  const [rating, setRating] = useState(existing?.rating ?? 0);
  const [comment, setComment] = useState(existing?.comment ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const t = useTranslations("courses");

  async function submit() {
    if (rating < 1) {
      setError(t("pickARating"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const saved = existing
        ? await updateCourseReview(courseId, { rating, comment })
        : await createCourseReview(courseId, { rating, comment });
      onSaved(saved);
    } catch (err) {
      setError(err instanceof CourseError ? err.message : t("somethingWrong"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <p className="text-sm font-medium text-foreground">
        {existing ? t("editYourReview") : t("writeAReview")}
      </p>
      <div className="mt-2">
        <StarInput value={rating} onChange={setRating} />
      </div>
      <textarea
        value={comment}
        onChange={(e) => setComment(e.target.value)}
        rows={3}
        maxLength={2000}
        placeholder={t("reviewPlaceholder")}
        className="mt-3 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
      />
      {error && <p className="mt-2 text-sm text-destructive">{error}</p>}
      <div className="mt-3 flex gap-2">
        <Button type="button" size="sm" disabled={busy} onClick={() => void submit()}>
          {busy ? t("saving") : t("submitReview")}
        </Button>
        {onCancel && (
          <Button type="button" size="sm" variant="ghost" disabled={busy} onClick={onCancel}>
            {t("cancel")}
          </Button>
        )}
      </div>
    </div>
  );
}
