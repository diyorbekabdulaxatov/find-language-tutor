"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import {
  AdminError,
  getPayoutBatch,
  type PayoutBatchDetail as Detail,
} from "@/features/admin/api";
import { PayoutBatchStatusBadge } from "./payout-batch-status-badge";

function fmt(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function PayoutBatchDetail({ id }: { id: string }) {
  const [data, setData] = useState<Detail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errMsg, setErrMsg] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const d = await getPayoutBatch(id);
        if (!alive) return;
        setData(d);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setErrMsg(
          err instanceof AdminError ? err.message : "Could not load that batch.",
        );
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id]);

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/admin/payouts"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        Payouts
      </Link>

      {state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : state === "error" || !data ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
      ) : (
        <>
          <div className="rounded-2xl border border-border bg-card p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="font-display text-2xl">
                  {formatMoney(data.total)}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {data.teacherCount}{" "}
                  {data.teacherCount === 1 ? "teacher" : "teachers"} ·{" "}
                  {data.lineCount} {data.lineCount === 1 ? "lesson" : "lessons"}
                </p>
              </div>
              <PayoutBatchStatusBadge status={data.status} />
            </div>
            <dl className="mt-4 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
              <div className="flex justify-between sm:block">
                <dt className="text-muted-foreground">Run by</dt>
                <dd>{data.createdBy.displayName}</dd>
              </div>
              <div className="flex justify-between sm:block">
                <dt className="text-muted-foreground">Started</dt>
                <dd>{fmt(data.createdAt)}</dd>
              </div>
              {data.completedAt && (
                <div className="flex justify-between sm:block">
                  <dt className="text-muted-foreground">Completed</dt>
                  <dd>{fmt(data.completedAt)}</dd>
                </div>
              )}
            </dl>
          </div>

          <div className="overflow-hidden rounded-2xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2 font-medium">Teacher</th>
                  <th className="px-4 py-2 font-medium">Lessons</th>
                  <th className="px-4 py-2 text-right font-medium">Amount</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {data.lines.map((l) => (
                  <tr key={l.teacher.slug}>
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/teachers/${l.teacher.slug}`}
                        className="font-medium text-primary hover:underline"
                      >
                        {l.teacher.displayName}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {l.lessonCount}
                    </td>
                    <td className="px-4 py-3 text-right font-medium">
                      {formatMoney(l.amount)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
