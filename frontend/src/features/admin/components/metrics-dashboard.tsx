"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Star } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { cn } from "@/lib/utils";
import { getMetrics, type AdminMetrics } from "@/features/admin/api";
import { ActivityPanel } from "./activity-panel";

export function MetricsDashboard() {
  const [data, setData] = useState<AdminMetrics | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const t = useTranslations("admin");
  const locale = useLocale();
  const n = (v: number) => v.toLocaleString(locale);
  const m = (v: Parameters<typeof formatMoney>[0]) => formatMoney(v, locale);

  useEffect(() => {
    let alive = true;
    getMetrics()
      .then((m) => {
        if (!alive) return;
        setData(m);
        setState("ready");
      })
      .catch(() => {
        if (alive) setState("error");
      });
    return () => {
      alive = false;
    };
  }, []);

  if (state === "loading") {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} className="h-24 animate-pulse bg-muted" />
        ))}
      </div>
    );
  }

  if (state === "error" || !data) {
    return (
      <p className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
        {t("couldNotLoadMetrics")}
      </p>
    );
  }

  const decided = data.bookingsCompleted + data.bookingsCancelled;
  const completionRate =
    decided === 0 ? null : Math.round((data.bookingsCompleted / decided) * 100);
  const netRevenue = {
    amountMinor: data.captured.amountMinor - data.refunded.amountMinor,
    currency: data.currency,
  };

  return (
    <div className="flex flex-col gap-8">
      <Section title={t("money")}>
        <Tile label={t("gmv")} value={m(data.gmv)} accent />
        <Tile label={t("collectedNet")} value={m(netRevenue)} />
        <Tile label={t("refundedToStudents")} value={m(data.refunded)} />
        <Tile
          label={t("owedToTeachers")}
          value={m(data.payoutsOwed)}
          href={data.payoutsOwed.amountMinor > 0 ? "/admin/payouts" : undefined}
        />
        <Tile label={t("paidToTeachers")} value={m(data.payoutsPaid)} />
      </Section>

      <ActivityPanel metrics={data} />

      <Section title={t("bookings")}>
        <Tile label={t("total")} value={n(data.bookingsTotal)} />
        <Tile label={t("last7")} value={n(data.bookingsThisWeek)} />
        <Tile label={t("upcoming")} value={n(data.bookingsUpcoming)} />
        <Tile label={t("completed")} value={n(data.bookingsCompleted)} />
        <Tile label={t("cancelled")} value={n(data.bookingsCancelled)} />
        <Tile
          label={t("completionRate")}
          value={completionRate === null ? "—" : `${completionRate}%`}
          hint={t("completionHint")}
        />
      </Section>

      <Section title={t("community")}>
        <Tile label={t("users")} value={n(data.usersTotal)} />
        <Tile label={t("newUsers7")} value={n(data.usersThisWeek)} />
        <Tile label={t("activeStudents")} value={n(data.activeStudents)} hint={t("activeStudentsHint")} />
        <Tile label={t("teachers")} value={n(data.teachersTotal)} />
        <Tile
          label={t("awaitingApproval")}
          value={n(data.teachersPending)}
          href={data.teachersPending > 0 ? "/admin/teachers?status=pending" : undefined}
          warn={data.teachersPending > 0}
        />
        <Tile label={t("verifiedTeachers")} value={n(data.teachersVerified)} />
        <Tile
          label={t("averageRating")}
          value={data.averageRating > 0 ? data.averageRating.toFixed(1) : "—"}
          icon={data.averageRating > 0 ? <Star className="size-4 fill-star text-star" /> : undefined}
        />
        <Tile label={t("visibleReviews")} value={n(data.reviewsVisible)} href="/admin/reviews" />
        <Tile
          label={t("openDisputes")}
          value={n(data.disputesOpen)}
          href={data.disputesOpen > 0 ? "/admin/disputes" : undefined}
          warn={data.disputesOpen > 0}
        />
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h2 className="border-b border-border pb-2 font-display text-xl">{title}</h2>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{children}</div>
    </section>
  );
}

function Tile({
  label,
  value,
  hint,
  href,
  icon,
  accent,
  warn,
}: {
  label: string;
  value: string;
  hint?: string;
  href?: string;
  icon?: React.ReactNode;
  accent?: boolean;
  warn?: boolean;
}) {
  const t = useTranslations("admin");
  const inner = (
    <div
      className={cn(
        "h-full border p-4 transition-colors",
        accent
          ? "border-foreground bg-accent"
          : warn
            ? "border-star/50 bg-star/10"
            : "border-border bg-card",
        href && "hover:bg-muted",
      )}
    >
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 flex items-center gap-1.5 font-display text-2xl">
        {icon}
        {value}
      </div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
      {href && <div className="mt-1 text-xs font-bold text-link">{t("view")}</div>}
    </div>
  );
  return href ? (
    <Link href={href} className="block">
      {inner}
    </Link>
  ) : (
    inner
  );
}
