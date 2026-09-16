"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
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

const STATE_STYLE: Record<
  EarningsSummary["lessons"][number]["state"],
  { label: "stateHeld" | "stateAvailable" | "statePaid" | "stateReversed"; className: string }
> = {
  held: { label: "stateHeld", className: "bg-star/15 text-star" },
  available: { label: "stateAvailable", className: "bg-primary/15 text-primary" },
  paid: { label: "statePaid", className: "bg-mint/15 text-mint" },
  reversed: { label: "stateReversed", className: "bg-destructive/10 text-destructive" },
};

function formatClearingDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
  }).format(new Date(iso));
}

export function EarningsPanel() {
  const [data, setData] = useState<EarningsSummary | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const viewerTz = viewerTimezone();
  const t = useTranslations("dashboard");
  const locale = useLocale();

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
        {t("earningsCouldNotLoad")}
      </p>
    );
  }

  if (!data || data.lessons.length === 0) {
    return (
      <p className="rounded-xl bg-muted px-4 py-10 text-center text-sm text-muted-foreground">
        {t("noEarnings")}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="grid gap-3 sm:grid-cols-2">
        <Stat label={t("totalEarned")} value={formatMoney(data.totalEarned, locale)} />
        <Stat label={t("held")} value={formatMoney(data.held, locale)} hint={t("heldHint")} />
        <Stat label={t("available")} value={formatMoney(data.available, locale)} accent hint={t("availableHint")} />
        <Stat label={t("paidOut")} value={formatMoney(data.paid, locale)} />
      </div>

      <div className="overflow-hidden rounded-2xl border border-border">
        <table className="w-full text-sm">
          <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
            <tr>
              <th className="px-4 py-2 font-medium">{t("colStudent")}</th>
              <th className="px-4 py-2 font-medium">{t("colLesson")}</th>
              <th className="px-4 py-2 text-right font-medium">{t("colAmount")}</th>
              <th className="px-4 py-2 text-right font-medium">{t("colStatus")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {data.lessons.map((l) => (
              <tr key={l.bookingId ?? l.courseEnrollmentId}>
                <td className="px-4 py-3">{l.studentDisplayName}</td>
                <td className="px-4 py-3 text-muted-foreground">
                  {l.bookingId ? (
                    <Link
                      href={`/bookings/${l.bookingId}`}
                      className="hover:text-foreground hover:underline"
                    >
                      {formatDayLabel(l.startAt, viewerTz, locale)}
                    </Link>
                  ) : (
                    <span>{t("courseSale", { title: l.courseTitle ?? "" })}</span>
                  )}
                </td>
                <td className="px-4 py-3 text-right font-medium">
                  {formatMoney(l.amount, locale)}
                </td>
                <td className="px-4 py-3 text-right">
                  <span
                    className={cn(
                      "inline-flex rounded-full px-2 py-0.5 text-xs font-semibold",
                      STATE_STYLE[l.state].className,
                    )}
                  >
                    {t(STATE_STYLE[l.state].label)}
                  </span>
                  {l.state === "held" && (
                    <div className="mt-0.5 text-xs text-muted-foreground">
                      {t("clears", { date: formatClearingDate(l.availableAt, locale) })}
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
