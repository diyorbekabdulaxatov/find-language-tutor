"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import {
  AdminError,
  listDisputes,
  resolveDispute,
  type AdminDisputeRow,
  type DisputeQueueStatus,
} from "@/features/admin/api";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;
const FILTERS: { value: DisputeQueueStatus; label: FilterKey }[] = [
  { value: "open", label: "filterOpen" },
  { value: "resolved", label: "filterResolved" },
  { value: "rejected", label: "filterRejected" },
  { value: "all", label: "filterAll" },
];
type FilterKey = "filterOpen" | "filterResolved" | "filterRejected" | "filterAll";
const STATUS_LABEL = { open: "disputeOpen", resolved: "disputeResolved", rejected: "disputeRejected" } as const;

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function DisputesQueue() {
  const [status, setStatus] = useState<DisputeQueueStatus>("open");
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState<AdminDisputeRow[]>([]);
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
        const res = await listDisputes({ status, page });
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
  }, [status, page, reloadKey]);

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => {
              setPage(1);
              setStatus(f.value);
            }}
            className={cn(
              "h-9 rounded-md border px-3 text-sm font-bold transition-colors",
              status === f.value
                ? "border-foreground bg-foreground text-background"
                : "border-border hover:bg-accent",
            )}
          >
            {t(f.label)}
          </button>
        ))}
      </div>

      {state === "error" ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoadDisputes")}
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse bg-muted" />
      ) : rows.length === 0 ? (
        <p className="border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
          {t("nothingHere")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {rows.map((d) => (
            <DisputeCard
              key={d.id}
              dispute={d}
              onResolved={() => setReloadKey((k) => k + 1)}
            />
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

function DisputeCard({
  dispute: d,
  onResolved,
}: {
  dispute: AdminDisputeRow;
  onResolved: () => void;
}) {
  const [outcome, setOutcome] = useState<"resolved" | "rejected" | null>(null);
  const [resolution, setResolution] = useState("");
  const [refund, setRefund] = useState(true);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const t = useTranslations("admin");
  const locale = useLocale();

  async function submit() {
    if (!outcome) return;
    setBusy(true);
    setErr(null);
    try {
      await resolveDispute(d.id, {
        outcome,
        resolution: resolution.trim(),
        refund: outcome === "resolved" && refund,
      });
      onResolved();
    } catch (e) {
      setErr(e instanceof AdminError ? e.message : t("couldNotResolve"));
      setBusy(false);
    }
  }

  return (
    <li className="border border-border bg-card p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="text-sm">
            <Link
              href={`/admin/bookings/${d.booking.id}`}
              className="font-bold text-link hover:underline"
            >
              {d.booking.teacher.displayName} × {d.booking.student.displayName}
            </Link>{" "}
            <span className="text-muted-foreground">
              · {fmt(d.booking.startAt, locale)} · {formatMoney(d.booking.price, locale)}
            </span>
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("raisedByOn", { name: d.raisedBy.displayName, date: fmt(d.createdAt, locale) })}
          </p>
        </div>
        <span
          className={cn(
            "rounded-sm px-2 py-0.5 text-xs font-bold",
            d.status === "open"
              ? "bg-accent text-accent-foreground"
              : "bg-muted text-muted-foreground",
          )}
        >
          {t(STATUS_LABEL[d.status])}
        </span>
      </div>

      <p className="mt-3 whitespace-pre-wrap text-sm">{d.reason}</p>

      {d.resolution && (
        <p className="mt-3 rounded-lg bg-muted px-3 py-2 text-sm">
          <span className="font-bold">
            {d.resolvedBy?.displayName ?? t("moderator")}:
          </span>{" "}
          {d.resolution}
        </p>
      )}

      {d.status === "open" && (
        <div className="mt-4 border-t border-border pt-4">
          {outcome === null ? (
            <div className="flex gap-2">
              <Button size="sm" onClick={() => setOutcome("resolved")}>
                {t("resolveSide")}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setOutcome("rejected")}
              >
                {t("reject")}
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <label className="text-xs font-bold text-muted-foreground">
                {t("closingNote", { outcome: t(outcome === "resolved" ? "outcomeResolved" : "outcomeRejected") })}
              </label>
              <textarea
                rows={2}
                value={resolution}
                onChange={(e) => setResolution(e.target.value)}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              {outcome === "resolved" && (
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={refund}
                    onChange={(e) => setRefund(e.target.checked)}
                    className="accent-primary"
                  />
                  {t("refundBooking")}
                </label>
              )}
              <div className="flex gap-2">
                <Button
                  size="sm"
                  disabled={busy || resolution.trim() === ""}
                  onClick={submit}
                >
                  {busy
                    ? t("saving")
                    : t("confirmOutcome", { outcome: t(outcome === "resolved" ? "outcomeResolved" : "outcomeRejected") })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setOutcome(null);
                    setResolution("");
                  }}
                >
                  {t("back")}
                </Button>
              </div>
            </div>
          )}
          {err && (
            <p className="mt-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {err}
            </p>
          )}
        </div>
      )}
    </li>
  );
}
