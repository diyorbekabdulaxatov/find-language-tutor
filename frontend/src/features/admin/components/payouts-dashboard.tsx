"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import {
  AdminError,
  getPayoutDashboard,
  runPayout,
  type PayoutDashboard,
} from "@/features/admin/api";
import { PERMISSIONS } from "@/features/admin/permissions";
import { useCan } from "@/features/admin/use-can";
import { Button } from "@/components/ui/button";
import { PayoutBatchStatusBadge } from "./payout-batch-status-badge";

const PAGE_SIZE = 20;

function fmtDate(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function PayoutsDashboard() {
  const { can } = useCan();
  const canRun = can(PERMISSIONS.payoutsRun);

  const [page, setPage] = useState(1);
  const [data, setData] = useState<PayoutDashboard | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const res = await getPayoutDashboard(page);
        if (!alive) return;
        setData(res);
        setState("ready");
      } catch {
        if (alive) setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [page, reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  if (state === "loading" && !data) {
    return <div className="h-72 animate-pulse rounded-2xl bg-muted" />;
  }

  if (state === "error" || !data) {
    return (
      <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
        Could not load the payout dashboard. Refresh to try again.
      </p>
    );
  }

  const { owed, totals, batches, batchesTotal } = data;
  const lastPage = Math.max(1, Math.ceil(batchesTotal / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-8">
      <section className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Stat label="Available to pay" value={formatMoney(totals.available)} accent />
          <Stat
            label="Held"
            value={formatMoney(totals.held)}
            hint="Inside the clearing window"
          />
          <Stat label="Paid out to date" value={formatMoney(totals.paid)} />
        </div>

        {canRun && (
          <RunPayoutCard
            owedCount={owed.length}
            available={formatMoney(totals.available)}
            onDone={reload}
          />
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-display text-xl">Owed now</h2>
        {owed.length === 0 ? (
          <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
            Nothing has cleared its holding period. Nothing to pay out.
          </p>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2 font-medium">Teacher</th>
                  <th className="px-4 py-2 font-medium">Cleared since</th>
                  <th className="px-4 py-2 text-right font-medium">Amount</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {owed.map((o) => (
                  <tr key={o.teacher.slug}>
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/teachers/${o.teacher.slug}`}
                        className="font-medium text-primary hover:underline"
                      >
                        {o.teacher.displayName}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {fmtDate(o.oldestAvailableAt)}
                    </td>
                    <td className="px-4 py-3 text-right font-medium">
                      {formatMoney(o.available)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-display text-xl">Past runs</h2>
        {batches.length === 0 ? (
          <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
            No payout runs yet.
          </p>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2 font-medium">Run</th>
                  <th className="px-4 py-2 font-medium">By</th>
                  <th className="px-4 py-2 font-medium">Teachers</th>
                  <th className="px-4 py-2 font-medium">Lessons</th>
                  <th className="px-4 py-2 text-right font-medium">Total</th>
                  <th className="px-4 py-2 text-right font-medium">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {batches.map((b) => (
                  <tr key={b.id}>
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/payouts/batches/${b.id}`}
                        className="font-medium text-primary hover:underline"
                      >
                        {fmtDate(b.createdAt)}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {b.createdBy.displayName}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {b.teacherCount}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {b.lineCount}
                    </td>
                    <td className="px-4 py-3 text-right font-medium">
                      {formatMoney(b.total)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <PayoutBatchStatusBadge status={b.status} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {batchesTotal > PAGE_SIZE && (
          <div className="flex items-center justify-between text-sm">
            <span className="text-muted-foreground">
              {batchesTotal.toLocaleString("en-US")} · page {page} of {lastPage}
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
      </section>
    </div>
  );
}

function RunPayoutCard({
  owedCount,
  available,
  onDone,
}: {
  owedCount: number;
  available: string;
  onDone: () => void;
}) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const nothingToPay = owedCount === 0;

  async function run() {
    setBusy(true);
    setErr(null);
    try {
      const batch = await runPayout();
      setResult(
        `Paid ${formatMoney(batch.total)} to ${batch.teacherCount} ${
          batch.teacherCount === 1 ? "teacher" : "teachers"
        } across ${batch.lineCount} ${
          batch.lineCount === 1 ? "lesson" : "lessons"
        }.`,
      );
      setConfirming(false);
      onDone();
    } catch (e) {
      if (e instanceof AdminError && e.code === "nothing_to_pay") {
        setErr("Nothing has cleared its holding period since you last checked.");
      } else {
        setErr(
          e instanceof AdminError ? e.message : "Could not run the payout. Try again.",
        );
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-2xl border border-primary/30 bg-primary/5 p-5">
      {result ? (
        <p className="text-sm">
          <span className="font-medium text-primary">Done.</span> {result}
        </p>
      ) : !confirming ? (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            {nothingToPay
              ? "No earnings have cleared their holding period yet."
              : `Pay every teacher what has cleared — ${available} across ${owedCount} ${
                  owedCount === 1 ? "teacher" : "teachers"
                }.`}
          </p>
          <Button
            size="sm"
            disabled={nothingToPay}
            onClick={() => setConfirming(true)}
          >
            Run payout
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <p className="text-sm">
            Settle <span className="font-medium">{available}</span> to {owedCount}{" "}
            {owedCount === 1 ? "teacher" : "teachers"} now? This can&apos;t be undone.
          </p>
          <div className="flex gap-2">
            <Button size="sm" disabled={busy} onClick={run}>
              {busy ? "Running…" : "Yes, pay now"}
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => setConfirming(false)}
            >
              Cancel
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
