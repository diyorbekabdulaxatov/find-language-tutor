"use client";

import { useEffect, useState } from "react";
import {
  getTeacherReviews,
  ReviewError,
  type Review,
} from "@/features/reviews/api";
import { Stars } from "./star-rating";

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

/**
 * Reviews on a teacher's profile. A client island: the aggregate rating is
 * already server-rendered in the masthead; this lazy-loads the list.
 */
export function ReviewsSection({
  slug,
  reviewCount,
}: {
  slug: string;
  reviewCount: number;
}) {
  const [reviews, setReviews] = useState<Review[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

  useEffect(() => {
    let alive = true;
    async function load() {
      if (page === 1) setState("loading");
      try {
        const res = await getTeacherReviews(slug, page);
        if (!alive) return;
        setReviews((prev) => (page === 1 ? res.reviews : [...prev, ...res.reviews]));
        setTotal(res.total);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        if (err instanceof ReviewError && err.status === 404) {
          setState("ready");
        } else {
          setState("error");
        }
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [slug, page]);

  if (state === "error") return null;

  return (
    <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
      <h2 className="font-display text-xl">
        Reviews{" "}
        <span className="text-muted-foreground">
          ({reviewCount.toLocaleString("en-US")})
        </span>
      </h2>

      {state === "loading" && (
        <div className="mt-4 space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-16 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      )}

      {state === "ready" && reviews.length === 0 && (
        <p className="mt-3 text-sm text-muted-foreground">No written reviews yet.</p>
      )}

      {reviews.length > 0 && (
        <ul className="mt-4 flex flex-col divide-y divide-border">
          {reviews.map((r) => (
            <li key={r.id} className="py-4 first:pt-0">
              <div className="flex items-center justify-between gap-3">
                <span className="font-medium">{r.studentDisplayName}</span>
                <span className="text-xs text-muted-foreground">
                  {formatDate(r.createdAt)}
                </span>
              </div>
              <Stars value={r.rating} className="mt-1" />
              {r.comment && (
                <p className="mt-2 text-sm leading-relaxed text-foreground/90">
                  {r.comment}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}

      {reviews.length < total && (
        <button
          type="button"
          onClick={() => setPage((p) => p + 1)}
          className="mt-4 text-sm font-medium text-primary hover:underline"
        >
          Show more reviews
        </button>
      )}
    </section>
  );
}
