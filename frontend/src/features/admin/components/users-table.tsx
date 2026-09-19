"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { intlLocale } from "@/lib/i18n";
import { AdminError, listUsers, type AdminUserRow } from "@/features/admin/api";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

function formatDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function UsersTable() {
  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState<AdminUserRow[]>([]);
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
        const res = await listUsers({ q: debounced || undefined, page });
        if (!alive) return;
        setRows(res.items);
        setTotal(res.total);
        setState("ready");
      } catch (err) {
        if (alive) setState(err instanceof AdminError ? "error" : "error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [debounced, page]);

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <div className="flex flex-col gap-4">
      <Input
        placeholder={t("searchUsers")}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-sm"
      />

      {state === "error" ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoadUsers")}
        </p>
      ) : (
        <div className="overflow-x-auto border border-border">
          <table className="w-full min-w-[36rem] text-sm">
            <thead className="bg-muted text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-2.5 font-bold">{t("colUser")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colTeacher")}</th>
                <th className="px-4 py-2.5 text-right font-bold">{t("colBookings")}</th>
                <th className="px-4 py-2.5 text-right font-bold">{t("colJoined")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {state === "loading" &&
                Array.from({ length: 6 }).map((_, i) => (
                  <tr key={i}>
                    <td colSpan={4} className="px-4 py-4">
                      <div className="h-4 animate-pulse rounded bg-muted" />
                    </td>
                  </tr>
                ))}
              {state === "ready" && rows.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-muted-foreground">
                    {t("noUsersMatch")}
                  </td>
                </tr>
              )}
              {state === "ready" &&
                rows.map((u) => (
                  <tr key={u.id} className="hover:bg-muted">
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/users/${u.id}`}
                        className="font-bold text-link hover:underline"
                      >
                        {u.displayName}
                      </Link>
                      <div className="text-xs text-muted-foreground">{u.email}</div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {u.isTeacher ? t("yes") : "—"}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums">
                      {u.bookingCount}
                    </td>
                    <td className="px-4 py-3 text-right text-muted-foreground">
                      {formatDate(u.createdAt, locale)}
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
            {t("usersPageOf", { total: total.toLocaleString(locale), page, last: lastPage })}
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
