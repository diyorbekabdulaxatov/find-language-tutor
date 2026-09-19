"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ShieldAlert } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import {
  listAdminBookings,
  type AdminBookingRow,
  type AdminBookingStatus,
} from "@/features/admin/api";
import { BookingStatusBadge } from "@/features/bookings/components/booking-status-badge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;
const FILTERS: { value: "" | AdminBookingStatus; label: FilterKey }[] = [
  { value: "", label: "filterAll" },
  { value: "pending_payment", label: "filterPending" },
  { value: "confirmed", label: "filterConfirmed" },
  { value: "completed", label: "filterCompleted" },
  { value: "cancelled", label: "filterCancelled" },
];
type FilterKey = "filterAll" | "filterPending" | "filterConfirmed" | "filterCompleted" | "filterCancelled";

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function BookingsTable() {
  const router = useRouter();
  const params = useSearchParams();
  const statusParam = (params.get("status") ?? "") as "" | AdminBookingStatus;

  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState<AdminBookingRow[]>([]);
  const [total, setTotal] = useState(0);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const t = useTranslations("admin");
  const locale = useLocale();

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebounced(q.trim());
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [q]);

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const res = await listAdminBookings({
          status: statusParam || undefined,
          q: debounced || undefined,
          page,
        });
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
  }, [statusParam, debounced, page]);

  function setStatus(next: "" | AdminBookingStatus) {
    setPage(1);
    router.replace(next ? `/admin/bookings?status=${next}` : "/admin/bookings", {
      scroll: false,
    });
  }

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {FILTERS.map((f) => (
          <button
            key={f.value || "all"}
            onClick={() => setStatus(f.value)}
            className={cn(
              "h-9 rounded-md border px-3 text-sm font-bold transition-colors",
              statusParam === f.value
                ? "border-foreground bg-foreground text-background"
                : "border-border hover:bg-accent",
            )}
          >
            {t(f.label)}
          </button>
        ))}
      </div>

      <Input
        placeholder={t("searchBookings")}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-sm"
      />

      {state === "error" ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoadBookings")}
        </p>
      ) : (
        <div className="overflow-x-auto border border-border">
          <table className="w-full min-w-[44rem] text-sm">
            <thead className="bg-muted text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-2.5 font-bold">{t("colLesson")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colTeacher")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colStudent")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colStatus")}</th>
                <th className="px-4 py-2.5 text-right font-bold">{t("colPrice")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {state === "loading" &&
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i}>
                    <td colSpan={5} className="px-4 py-4">
                      <div className="h-4 animate-pulse rounded bg-muted" />
                    </td>
                  </tr>
                ))}
              {state === "ready" && rows.length === 0 && (
                <tr>
                  <td
                    colSpan={5}
                    className="px-4 py-8 text-center text-muted-foreground"
                  >
                    {t("noBookingsMatch")}
                  </td>
                </tr>
              )}
              {state === "ready" &&
                rows.map((b) => (
                  <tr key={b.id} className="hover:bg-muted">
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/bookings/${b.id}`}
                        className="inline-flex items-center gap-1.5 font-bold text-link hover:underline"
                      >
                        {fmt(b.startAt, locale)}
                        {b.hasOpenDispute && (
                          <ShieldAlert className="size-4 text-link" />
                        )}
                      </Link>
                    </td>
                    <td className="px-4 py-3">{b.teacher.displayName}</td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {b.student.displayName}
                    </td>
                    <td className="px-4 py-3">
                      <BookingStatusBadge status={b.status} />
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums">
                      {formatMoney(b.price, locale)}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>
        </div>
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
