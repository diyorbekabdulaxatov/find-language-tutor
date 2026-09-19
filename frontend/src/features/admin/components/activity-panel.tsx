"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import {
  ACTIVITY_WINDOWS,
  getActivity,
  type ActivityWindow,
  type AdminActivity,
  type AdminMetrics,
} from "@/features/admin/api";
import { TrendChart } from "./charts/trend-chart";
import { ShareBar } from "./charts/share-bar";

/**
 * The moving part of the dashboard: three single-measure trends over a chosen
 * window, the booking lifecycle as one share bar, and who earned the most in
 * the window. Each trend is its own chart rather than one chart with three
 * y-scales — counts and so'm don't share an axis.
 */
export function ActivityPanel({ metrics }: { metrics: AdminMetrics }) {
  const [days, setDays] = useState<ActivityWindow>(30);
  const [data, setData] = useState<AdminActivity | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const t = useTranslations("admin");
  const locale = useLocale();

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const a = await getActivity(days);
        if (!alive) return;
        setData(a);
        setState("ready");
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [days]);

  const n = (v: number) => v.toLocaleString(intlLocale(locale));
  const day = (iso: string) =>
    new Date(`${iso}T00:00:00Z`).toLocaleDateString(intlLocale(locale), {
      day: "numeric",
      month: "short",
      timeZone: "UTC",
    });

  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-2">
        <h2 className="font-display text-xl">{t("activity")}</h2>
        <div className="flex gap-2" role="group" aria-label={t("window")}>
          {ACTIVITY_WINDOWS.map((w) => (
            <button
              key={w}
              type="button"
              onClick={() => setDays(w)}
              aria-pressed={days === w}
              className={cn(
                "h-8 rounded-md border px-3 text-xs font-bold transition-colors",
                days === w
                  ? "border-foreground bg-foreground text-background"
                  : "border-border hover:bg-accent",
              )}
            >
              {t("lastDays", { count: w })}
            </button>
          ))}
        </div>
      </div>

      {state === "error" && (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("activityCouldNotLoad")}
        </p>
      )}

      {state === "loading" && (
        <div className="grid gap-4 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-[268px] animate-pulse bg-muted" />
          ))}
        </div>
      )}

      {state === "ready" && data && (
        <>
          <div className="grid gap-4 lg:grid-cols-3">
            <TrendChart
              title={t("bookingsPerDay")}
              points={data.points.map((p) => ({ day: p.day, value: p.bookings }))}
              format={n}
              formatDay={day}
            />
            <TrendChart
              title={t("gmvPerDay")}
              points={data.points.map((p) => ({ day: p.day, value: p.gmv.amountMinor }))}
              format={(v) => formatMoney({ amountMinor: v, currency: data.currency }, locale)}
              formatDay={day}
            />
            <TrendChart
              title={t("signupsPerDay")}
              points={data.points.map((p) => ({ day: p.day, value: p.signups }))}
              format={n}
              formatDay={day}
            />
          </div>

          <div className="mt-2 grid gap-4 lg:grid-cols-2">
            <ShareBar
              title={t("bookingMix")}
              empty={t("noBookingsYet")}
              formatValue={n}
              shares={[
                {
                  key: "pending_payment",
                  label: t("mixPending"),
                  value: metrics.bookingsPending,
                  color: "var(--chart-life-1)",
                },
                {
                  key: "confirmed",
                  label: t("mixConfirmed"),
                  value: metrics.bookingsConfirmed,
                  color: "var(--chart-life-2)",
                },
                {
                  key: "completed",
                  label: t("mixCompleted"),
                  value: metrics.bookingsCompleted,
                  color: "var(--chart-life-3)",
                },
                {
                  key: "cancelled",
                  label: t("mixCancelled"),
                  value: metrics.bookingsCancelled,
                  color: "var(--chart-cancelled)",
                },
              ]}
            />

            <TopEarners rows={data.topTeachers} locale={locale} />
          </div>
        </>
      )}
    </section>
  );
}

/** Identity + magnitude with names on it: a table, with the bar as the scale. */
function TopEarners({
  rows,
  locale,
}: {
  rows: AdminActivity["topTeachers"];
  locale: string;
}) {
  const t = useTranslations("admin");
  const max = rows.reduce((m, r) => Math.max(m, r.gmv.amountMinor), 0);

  return (
    <figure className="m-0 border border-border bg-card p-4">
      <figcaption className="text-sm font-bold">{t("topEarners")}</figcaption>
      {rows.length === 0 ? (
        <p className="mt-3 text-sm text-muted-foreground">{t("noEarnersYet")}</p>
      ) : (
        <ul className="mt-3 flex flex-col gap-3">
          {rows.map((r) => (
            <li key={r.slug}>
              <div className="flex items-baseline justify-between gap-3 text-sm">
                <Link
                  href={`/admin/teachers/${r.slug}`}
                  className="truncate font-bold text-link hover:underline"
                >
                  {r.displayName}
                </Link>
                <span className="shrink-0 font-bold">{formatMoney(r.gmv, locale)}</span>
              </div>
              <div className="mt-1 flex items-center gap-2">
                <div className="h-1.5 flex-1 bg-muted">
                  <div
                    className="h-full rounded-[2px]"
                    style={{
                      width: max > 0 ? `${(r.gmv.amountMinor / max) * 100}%` : "0%",
                      background: "var(--chart-series)",
                    }}
                  />
                </div>
                <span className="shrink-0 text-xs text-muted-foreground">
                  {t("lessonsCount", { count: r.lessons })}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </figure>
  );
}
