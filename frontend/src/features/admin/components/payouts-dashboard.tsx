"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
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

function fmtDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
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
  const t = useTranslations("admin");
  const locale = useLocale();

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
        {t("couldNotLoadPayouts")}
      </p>
    );
  }

  const { owed, totals, batches, batchesTotal } = data;
  const lastPage = Math.max(1, Math.ceil(batchesTotal / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-8">
      <section className="flex flex-col gap-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Stat label={t("availableToPay")} value={formatMoney(totals.available, locale)} accent />
          <Stat label={t("held")} value={formatMoney(totals.held, locale)} hint={t("heldHint")} />
          <Stat label={t("paidToDate")} value={formatMoney(totals.paid, locale)} />
        </div>

        {canRun && (
          <RunPayoutCard
            owedCount={owed.length}
            available={formatMoney(totals.available, locale)}
            onDone={reload}
          />
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-display text-xl">{t("owedNow")}</h2>
        {owed.length === 0 ? (
          <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
            {t("nothingCleared")}
          </p>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2 font-medium">{t("colTeacher")}</th>
                  <th className="px-4 py-2 font-medium">{t("colClearedSince")}</th>
                  <th className="px-4 py-2 text-right font-medium">{t("colAmount")}</th>
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
                      {fmtDate(o.oldestAvailableAt, locale)}
                    </td>
                    <td className="px-4 py-3 text-right font-medium">
                      {formatMoney(o.available, locale)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="font-display text-xl">{t("pastRuns")}</h2>
        {batches.length === 0 ? (
          <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
            {t("noRuns")}
          </p>
        ) : (
          <div className="overflow-hidden rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2 font-medium">{t("colRun")}</th>
                  <th className="px-4 py-2 font-medium">{t("colBy")}</th>
                  <th className="px-4 py-2 font-medium">{t("colTeachers")}</th>
                  <th className="px-4 py-2 font-medium">{t("colLessons")}</th>
                  <th className="px-4 py-2 text-right font-medium">{t("colTotal")}</th>
                  <th className="px-4 py-2 text-right font-medium">{t("colStatus")}</th>
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
                        {fmtDate(b.createdAt, locale)}
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
                      {formatMoney(b.total, locale)}
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
              {t("pageOf", { total: batchesTotal.toLocaleString(locale), page, last: lastPage })}
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
  const t = useTranslations("admin");
  const locale = useLocale();

  const nothingToPay = owedCount === 0;

  async function run() {
    setBusy(true);
    setErr(null);
    try {
      const batch = await runPayout();
      setResult(
        t("payoutDone", {
          total: formatMoney(batch.total, locale),
          teachers: batch.teacherCount,
          lessons: batch.lineCount,
        }),
      );
      setConfirming(false);
      onDone();
    } catch (e) {
      if (e instanceof AdminError && e.code === "nothing_to_pay") {
        setErr(t("nothingClearedSince"));
      } else {
        setErr(e instanceof AdminError ? e.message : t("couldNotRunPayout"));
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-2xl border border-primary/30 bg-primary/5 p-5">
      {result ? (
        <p className="text-sm">
          <span className="font-medium text-primary">{t("done")}</span> {result}
        </p>
      ) : !confirming ? (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            {nothingToPay ? t("noEarningsCleared") : t("payEveryone", { available, count: owedCount })}
          </p>
          <Button
            size="sm"
            disabled={nothingToPay}
            onClick={() => setConfirming(true)}
          >
            {t("runPayout")}
          </Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          <p className="text-sm">
            {t.rich("settleConfirm", {
              available,
              count: owedCount,
              b: (chunks) => <span className="font-medium">{chunks}</span>,
            })}
          </p>
          <div className="flex gap-2">
            <Button size="sm" disabled={busy} onClick={run}>
              {busy ? t("running") : t("yesPayNow")}
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => setConfirming(false)}
            >
              {t("cancel")}
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
