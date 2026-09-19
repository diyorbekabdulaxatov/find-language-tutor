"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  getTeacherReviews,
  ReviewError,
  type Review,
} from "@/features/reviews/api";
import { Stars } from "./star-rating";
import { intlLocale } from "@/lib/i18n";

function formatDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
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
  const t = useTranslations("reviews");
  const locale = useLocale();

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
    <section>
      <h2 className="font-display text-2xl">
        {t("title")}{" "}
        <span className="text-muted-foreground">
          ({reviewCount.toLocaleString(locale)})
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
        <p className="mt-3 text-sm text-muted-foreground">{t("noneYet")}</p>
      )}

      {reviews.length > 0 && (
        <ul className="mt-4 divide-y divide-border border-t border-border">
          {reviews.map((r) => (
            <li key={r.id} className="flex gap-4 py-5">
              <span className="grid size-12 shrink-0 place-items-center rounded-full bg-ink text-base font-bold text-ink-foreground">
                {r.studentDisplayName
                  .split(/\s+/)
                  .filter(Boolean)
                  .slice(0, 2)
                  .map((p) => p[0]?.toUpperCase() ?? "")
                  .join("")}
              </span>
              <div className="min-w-0 flex-1">
                <p className="text-base font-bold">{r.studentDisplayName}</p>
                <div className="mt-0.5 flex items-center gap-2">
                  <Stars value={r.rating} className="[&_svg]:size-3.5" />
                  <span className="text-xs text-muted-foreground">
                    {formatDate(r.createdAt, locale)}
                  </span>
                </div>
                {r.comment && (
                  <p className="mt-2 text-sm leading-relaxed text-foreground/90">{r.comment}</p>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}

      {reviews.length < total && (
        <button
          type="button"
          onClick={() => setPage((p) => p + 1)}
          className="mt-4 inline-flex h-10 items-center rounded-md border border-foreground bg-background px-3 text-sm font-bold text-foreground hover:bg-accent"
        >
          {t("showMore")}
        </button>
      )}
    </section>
  );
}
