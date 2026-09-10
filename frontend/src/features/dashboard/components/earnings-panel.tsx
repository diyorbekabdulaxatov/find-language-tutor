"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { formatMoney } from "@/lib/format";
import {
  BookingError,
  getEarnings,
  type EarningsSummary,
} from "@/features/bookings/api";
import {
  formatDayLabel,
  viewerTimezone,
} from "@/features/bookings/datetime";
import { cn } from "@/lib/utils";

const STATE_LABEL: Record<
  EarningsSummary["lessons"][number]["state"],
  { label: string; className: string }
> = {
  held: { label: "Held", className: "bg-star/15 text-star" },
  available: { label: "Available", className: "bg-primary/15 text-primary" },
  paid: { label: "Paid out", className: "bg-mint/15 text-mint" },
  reversed: { label: "Refunded", className: "bg-destructive/10 text-destructive" },
};

function formatClearingDate(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
  }).format(new Date(iso));
}

export function EarningsPanel() {
  const [data, setData] = useState<EarningsSummary | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const viewerTz = viewerTimezone();

  useEffect(() => {
    let alive = true;
    getEarnings()
      .then((d) => {
        if (!alive) return;
        setData(d);
        setState("ready");
      })
      .catch((err) => {
        if (!alive) return;
        // no profile yet → treat as empty
        if (err instanceof BookingError && err.status === 404) {
          setState("ready");
          return;
        }
        setState("error");
      });
    return () => {
      alive = false;
    };
  }, []);

  if (state === "loading") {
    return <div className="h-64 animate-pulse rounded-2xl bg-muted" />;
  }

  if (state === "error") {
    return (
      <p className="rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">
        Could not load your earnings. Refresh to try again.
      </p>
    );
  }

  if (!data || data.lessons.length === 0) {
    return (
      <p className="rounded-xl bg-muted px-4 py-10 text-center text-sm text-muted-foreground">
        No earnings yet. Once a lesson you taught is marked complete, it shows
        up here.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-3 sm:grid-cols-2">
        <Stat label="Total earned" value={formatMoney(data.totalEarned)} />
        <Stat label="Held" value={formatMoney(data.held)} hint="Inside the clearing window" />
        <Stat label="Available" value={formatMoney(data.available)} accent hint="Waiting on the next payout" />
        <Stat label="Paid out" value={formatMoney(data.paid)} />
      </div>

      <div className="overflow-hidden rounded-2xl border border-border">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-4 py-2 font-medium">Student</th>
              <th className="px-4 py-2 font-medium">Lesson</th>
              <th className="px-4 py-2 text-right font-medium">Amount</th>
              <th className="px-4 py-2 text-right font-medium">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {data.lessons.map((l) => (
              <tr key={l.bookingId}>
                <td className="px-4 py-3">{l.studentDisplayName}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  <Link
                    href={`/bookings/${l.bookingId}`}
                    className="hover:text-foreground hover:underline"
                  >
                    {formatDayLabel(l.startAt, viewerTz)}
                  </Link>
                </td>
                <td className="px-4 py-3 text-right font-medium">
                  {formatMoney(l.amount)}
                </td>
                <td className="px-4 py-3 text-right">
                  <span
                    className={cn(
                      "inline-flex rounded-full px-2 py-0.5 text-xs font-semibold",
                      STATE_LABEL[l.state].className,
                    )}
                  >
                    {STATE_LABEL[l.state].label}
                  </span>
                  {l.state === "held" && (
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      clears {formatClearingDate(l.availableAt)}
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  hint,
  accent,
}: {
  label: string;
  value: string;
  hint?: string;
  accent?: boolean;
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border p-4",
        accent ? "border-primary/30 bg-primary/5" : "border-border bg-card",
      )}
    >
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 font-display text-2xl">{value}</div>
      {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
    </div>
  );
}
