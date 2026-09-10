"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { formatMoney } from "@/lib/format";
import { AdminError, getMetrics, type AdminMetrics } from "@/features/admin/api";

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
      .catch((err) => {
        if (!alive) return;
        setState(err instanceof AdminError ? "error" : "error");
      });
    return () => {
      alive = false;
    };
  }, []);

  if (state === "loading") {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => (
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

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <Tile label="GMV (confirmed + completed)" value={formatMoney(data.gmv)} accent />
      <Tile label="Bookings, total" value={data.bookingsTotal.toLocaleString("en-US")} />
      <Tile label="Bookings, last 7 days" value={data.bookingsThisWeek.toLocaleString("en-US")} />
      <Tile label="Users" value={data.usersTotal.toLocaleString("en-US")} />
      <Tile label="Teachers" value={data.teachersTotal.toLocaleString("en-US")} />
      <Tile
        label="Awaiting approval"
        value={data.teachersPending.toLocaleString("en-US")}
        href={data.teachersPending > 0 ? "/admin/teachers?status=pending" : undefined}
        warn={data.teachersPending > 0}
      />
    </div>
  );
}

function Tile({
  label,
  value,
  href,
  accent,
  warn,
}: {
  label: string;
  value: string;
  href?: string;
  accent?: boolean;
  warn?: boolean;
}) {
  const inner = (
    <div
      className={[
        "rounded-2xl border p-4 transition-colors",
        accent
          ? "border-primary/30 bg-primary/5"
          : warn
            ? "border-star/40 bg-star/5"
            : "border-border bg-card",
        href ? "hover:border-primary/50" : "",
      ].join(" ")}
    >
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 font-display text-2xl">{value}</div>
      {href && (
        <div className="mt-1 text-xs font-medium text-primary">Review →</div>
      )}
    </div>
  );
  return href ? <Link href={href}>{inner}</Link> : inner;
}
