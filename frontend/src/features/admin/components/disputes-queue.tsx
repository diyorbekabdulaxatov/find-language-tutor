"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import {
  AdminError,
  listDisputes,
  resolveDispute,
  type AdminDisputeRow,
  type DisputeQueueStatus,
} from "@/features/admin/api";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;
const FILTERS: { value: DisputeQueueStatus; label: string }[] = [
  { value: "open", label: "Open" },
  { value: "resolved", label: "Resolved" },
  { value: "rejected", label: "Rejected" },
  { value: "all", label: "All" },
];

function fmt(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
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
              "rounded-lg border px-3 py-1.5 text-sm transition-colors",
              status === f.value
                ? "border-primary bg-accent"
                : "border-border hover:bg-muted",
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load the dispute queue.
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : rows.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
          Nothing here.
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
            {total.toLocaleString("en-US")} · page {page} of {lastPage}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= lastPage}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
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
      setErr(
        e instanceof AdminError ? e.message : "Could not resolve. Try again.",
      );
      setBusy(false);
    }
  }

  return (
    <li className="rounded-2xl border border-border bg-card p-5">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="text-sm">
            <Link
              href={`/admin/bookings/${d.booking.id}`}
              className="font-medium text-primary hover:underline"
            >
              {d.booking.teacher.displayName} × {d.booking.student.displayName}
            </Link>{" "}
            <span className="text-muted-foreground">
              · {fmt(d.booking.startAt)} · {formatMoney(d.booking.price)}
            </span>
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Raised by {d.raisedBy.displayName} on {fmt(d.createdAt)}
          </p>
        </div>
        <span
          className={cn(
            "rounded-full px-2.5 py-1 text-xs font-semibold",
            d.status === "open"
              ? "bg-coral/15 text-coral"
              : "bg-muted text-muted-foreground",
          )}
        >
          {d.status}
        </span>
      </div>

      <p className="mt-3 whitespace-pre-wrap text-sm">{d.reason}</p>

      {d.resolution && (
        <p className="mt-3 rounded-lg bg-muted px-3 py-2 text-sm">
          <span className="font-medium">
            {d.resolvedBy?.displayName ?? "Moderator"}:
          </span>{" "}
          {d.resolution}
        </p>
      )}

      {d.status === "open" && (
        <div className="mt-4 border-t border-border pt-4">
          {outcome === null ? (
            <div className="flex gap-2">
              <Button size="sm" onClick={() => setOutcome("resolved")}>
                Resolve (side with them)
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={() => setOutcome("rejected")}
              >
                Reject
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <label className="text-xs font-medium text-muted-foreground">
                Closing note ({outcome})
              </label>
              <textarea
                rows={2}
                value={resolution}
                onChange={(e) => setResolution(e.target.value)}
                className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              {outcome === "resolved" && (
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={refund}
                    onChange={(e) => setRefund(e.target.checked)}
                    className="accent-primary"
                  />
                  Refund the booking&apos;s payment
                </label>
              )}
              <div className="flex gap-2">
                <Button
                  size="sm"
                  disabled={busy || resolution.trim() === ""}
                  onClick={submit}
                >
                  {busy ? "Saving…" : `Confirm ${outcome}`}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setOutcome(null);
                    setResolution("");
                  }}
                >
                  Back
                </Button>
              </div>
            </div>
          )}
          {err && (
            <p className="mt-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {err}
            </p>
          )}
        </div>
      )}
    </li>
  );
}
