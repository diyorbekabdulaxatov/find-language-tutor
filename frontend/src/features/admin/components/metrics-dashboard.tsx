"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Star } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { cn } from "@/lib/utils";
import { getMetrics, type AdminMetrics } from "@/features/admin/api";

const n = (v: number) => v.toLocaleString("en-US");

export function MetricsDashboard() {
  const [data, setData] = useState<AdminMetrics | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");

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
          <div key={i} className="h-24 animate-pulse rounded-2xl bg-muted" />
        ))}
      </div>
    );
  }

  if (state === "error" || !data) {
    return (
      <p className="rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">
        Could not load the metrics.
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
      <Section title="Money">
        <Tile label="GMV (confirmed + completed)" value={formatMoney(data.gmv)} accent />
        <Tile label="Collected, net of refunds" value={formatMoney(netRevenue)} />
        <Tile label="Refunded to students" value={formatMoney(data.refunded)} />
        <Tile
          label="Owed to teachers"
          value={formatMoney(data.payoutsOwed)}
          href={data.payoutsOwed.amountMinor > 0 ? "/admin/payouts" : undefined}
        />
        <Tile label="Paid to teachers" value={formatMoney(data.payoutsPaid)} />
      </Section>

      <Section title="Bookings">
        <Tile label="Total" value={n(data.bookingsTotal)} />
        <Tile label="Last 7 days" value={n(data.bookingsThisWeek)} />
        <Tile label="Upcoming" value={n(data.bookingsUpcoming)} />
        <Tile label="Completed" value={n(data.bookingsCompleted)} />
        <Tile label="Cancelled" value={n(data.bookingsCancelled)} />
        <Tile
          label="Completion rate"
          value={completionRate === null ? "—" : `${completionRate}%`}
          hint="completed ÷ (completed + cancelled)"
        />
      </Section>

      <Section title="Community & moderation">
        <Tile label="Users" value={n(data.usersTotal)} />
        <Tile label="New users, last 7 days" value={n(data.usersThisWeek)} />
        <Tile label="Active students" value={n(data.activeStudents)} hint="have booked ≥ 1 lesson" />
        <Tile label="Teachers" value={n(data.teachersTotal)} />
        <Tile
          label="Awaiting approval"
          value={n(data.teachersPending)}
          href={data.teachersPending > 0 ? "/admin/teachers?status=pending" : undefined}
          warn={data.teachersPending > 0}
        />
        <Tile label="Verified teachers" value={n(data.teachersVerified)} />
        <Tile
          label="Average rating"
          value={data.averageRating > 0 ? data.averageRating.toFixed(1) : "—"}
          icon={data.averageRating > 0 ? <Star className="size-4 fill-star text-star" /> : undefined}
        />
        <Tile label="Visible reviews" value={n(data.reviewsVisible)} href="/admin/reviews" />
        <Tile
          label="Open disputes"
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
      <h2 className="font-display text-xl">{title}</h2>
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
  const inner = (
    <div
      className={cn(
        "h-full rounded-2xl border p-4 transition-colors",
        accent
          ? "border-primary/30 bg-primary/5"
          : warn
            ? "border-star/40 bg-star/5"
            : "border-border bg-card",
        href && "hover:border-primary/50",
      )}
    >
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 flex items-center gap-1.5 font-display text-2xl">
        {icon}
        {value}
      </div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
      {href && <div className="mt-1 text-xs font-medium text-primary">View →</div>}
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
