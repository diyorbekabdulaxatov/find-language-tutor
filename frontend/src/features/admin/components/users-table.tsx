"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { AdminError, listUsers, type AdminUserRow } from "@/features/admin/api";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

function formatDate(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
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

  useEffect(() => {
    const t = setTimeout(() => {
      setDebounced(q.trim());
      setPage(1);
    }, 300);
    return () => clearTimeout(t);
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
        placeholder="Search by name or email…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-sm"
      />

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load users.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-border">
          <table className="w-full min-w-[36rem] text-sm">
            <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-2 font-medium">User</th>
                <th className="px-4 py-2 font-medium">Teacher</th>
                <th className="px-4 py-2 text-right font-medium">Bookings</th>
                <th className="px-4 py-2 text-right font-medium">Joined</th>
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
                    No users match.
                  </td>
                </tr>
              )}
              {state === "ready" &&
                rows.map((u) => (
                  <tr key={u.id} className="hover:bg-muted/40">
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/users/${u.id}`}
                        className="font-medium hover:underline"
                      >
                        {u.displayName}
                      </Link>
                      <div className="text-xs text-muted-foreground">{u.email}</div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">
                      {u.isTeacher ? "Yes" : "—"}
                    </td>
                    <td className="px-4 py-3 text-right tabular-nums">
                      {u.bookingCount}
                    </td>
                    <td className="px-4 py-3 text-right text-muted-foreground">
                      {formatDate(u.createdAt)}
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
            {total.toLocaleString("en-US")} users · page {page} of {lastPage}
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
