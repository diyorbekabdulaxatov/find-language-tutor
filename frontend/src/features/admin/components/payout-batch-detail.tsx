"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import {
  AdminError,
  getPayoutBatch,
  type PayoutBatchDetail as Detail,
} from "@/features/admin/api";
import { PayoutBatchStatusBadge } from "./payout-batch-status-badge";

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
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
  const t = useTranslations("admin");
  const locale = useLocale();

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
        setErrMsg(err instanceof AdminError ? err.message : t("couldNotLoadBatch"));
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id, t]);

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/admin/payouts"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        {t("payouts")}
      </Link>

      {state === "loading" ? (
        <div className="h-64 animate-pulse bg-muted" />
      ) : state === "error" || !data ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
      ) : (
        <>
          <div className="border border-border bg-card p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2 className="font-display text-2xl">
                  {formatMoney(data.total, locale)}
                </h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("teachersLessons", { teachers: data.teacherCount, lessons: data.lineCount })}
                </p>
              </div>
              <PayoutBatchStatusBadge status={data.status} />
            </div>
            <dl className="mt-4 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
              <div className="flex justify-between sm:block">
                <dt className="text-muted-foreground">{t("runBy")}</dt>
                <dd>{data.createdBy.displayName}</dd>
              </div>
              <div className="flex justify-between sm:block">
                <dt className="text-muted-foreground">{t("started")}</dt>
                <dd>{fmt(data.createdAt, locale)}</dd>
              </div>
              {data.completedAt && (
                <div className="flex justify-between sm:block">
                  <dt className="text-muted-foreground">{t("completedAt")}</dt>
                  <dd>{fmt(data.completedAt, locale)}</dd>
                </div>
              )}
            </dl>
          </div>

          <div className="overflow-hidden border border-border">
            <table className="w-full text-sm">
              <thead className="bg-muted text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-4 py-2.5 font-bold">{t("colTeacher")}</th>
                  <th className="px-4 py-2.5 font-bold">{t("colLessons")}</th>
                  <th className="px-4 py-2.5 text-right font-bold">{t("colAmount")}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {data.lines.map((l) => (
                  <tr key={l.teacher.slug}>
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/teachers/${l.teacher.slug}`}
                        className="font-bold text-link hover:underline"
                      >
                        {l.teacher.displayName}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {l.lessonCount}
                    </td>
                    <td className="px-4 py-3 text-right font-bold">
                      {formatMoney(l.amount, locale)}
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
