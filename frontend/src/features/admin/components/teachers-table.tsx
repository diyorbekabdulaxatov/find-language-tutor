"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { BadgeCheck } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  AdminError,
  listTeachers,
  type AdminTeacherRow,
  type TeacherStatus,
} from "@/features/admin/api";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { TeacherStatusBadge } from "./teacher-status-badge";

const PAGE_SIZE = 20;
const FILTERS: { value: "" | TeacherStatus; label: string }[] = [
  { value: "", label: "All" },
  { value: "pending", label: "Pending" },
  { value: "approved", label: "Approved" },
  { value: "suspended", label: "Suspended" },
  { value: "rejected", label: "Rejected" },
];

function fmt(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function TeachersTable() {
  const router = useRouter();
  const params = useSearchParams();
  const statusParam = (params.get("status") ?? "") as "" | TeacherStatus;

  const [q, setQ] = useState("");
  const [debounced, setDebounced] = useState("");
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState<AdminTeacherRow[]>([]);
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
        const res = await listTeachers({
          status: statusParam || undefined,
          q: debounced || undefined,
          page,
        });
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
  }, [statusParam, debounced, page]);

  function setStatus(next: "" | TeacherStatus) {
    setPage(1);
    router.replace(next ? `/admin/teachers?status=${next}` : "/admin/teachers", {
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
              "rounded-lg border px-3 py-1.5 text-sm transition-colors",
              statusParam === f.value
                ? "border-primary bg-accent"
                : "border-border hover:bg-muted",
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      <Input
        placeholder="Search by name, slug or owner email…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-sm"
      />

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load teachers.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-border">
          <table className="w-full min-w-[40rem] text-sm">
            <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-4 py-2 font-medium">Teacher</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-4 py-2 font-medium">Owner</th>
                <th className="px-4 py-2 text-right font-medium">Created</th>
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
                    No teachers match.
                  </td>
                </tr>
              )}
              {state === "ready" &&
                rows.map((t) => (
                  <tr key={t.slug} className="hover:bg-muted/40">
                    <td className="px-4 py-3">
                      <Link
                        href={`/admin/teachers/${t.slug}`}
                        className="inline-flex items-center gap-1 font-medium hover:underline"
                      >
                        {t.displayName}
                        {t.verified && (
                          <BadgeCheck className="size-4 text-primary" />
                        )}
                      </Link>
                      <div className="max-w-xs truncate text-xs text-muted-foreground">
                        {t.headline}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <TeacherStatusBadge status={t.status} />
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {t.owner.email}
                    </td>
                    <td className="px-4 py-3 text-right text-muted-foreground">
                      {fmt(t.createdAt)}
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
