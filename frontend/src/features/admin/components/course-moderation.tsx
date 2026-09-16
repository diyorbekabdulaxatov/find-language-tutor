"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import {
  AdminError,
  listAdminCourses,
  suspendCourse,
  unsuspendCourse,
  type AdminCourse,
  type CourseModerationStatus,
} from "@/features/admin/api";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

const STATUS: { value: CourseModerationStatus | "all"; label: StatusKey }[] = [
  { value: "all", label: "courseAll" },
  { value: "draft", label: "courseDraft" },
  { value: "published", label: "coursePublished" },
  { value: "archived", label: "courseArchived" },
];
type StatusKey = "courseAll" | "courseDraft" | "coursePublished" | "courseArchived";

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function CourseModeration() {
  const [status, setStatus] = useState<CourseModerationStatus | "all">("all");
  const [suspendedOnly, setSuspendedOnly] = useState(false);
  const [teacher, setTeacher] = useState("");
  const [teacherQuery, setTeacherQuery] = useState("");
  const [page, setPage] = useState(1);

  const [rows, setRows] = useState<AdminCourse[]>([]);
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
        const res = await listAdminCourses({
          status: status === "all" ? undefined : status,
          suspended: suspendedOnly ? true : undefined,
          teacherSlug: teacherQuery || undefined,
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
  }, [status, suspendedOnly, teacherQuery, page, reloadKey]);

  const lastPage = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const reload = () => setReloadKey((k) => k + 1);

  function applyTeacher(e: React.FormEvent) {
    e.preventDefault();
    setPage(1);
    setTeacherQuery(teacher.trim());
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        {STATUS.map((f) => (
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
            {t(f.label)}
          </button>
        ))}

        <label className="ml-1 flex items-center gap-2 text-sm text-muted-foreground">
          <input
            type="checkbox"
            checked={suspendedOnly}
            onChange={(e) => {
              setPage(1);
              setSuspendedOnly(e.target.checked);
            }}
            className="accent-primary"
          />
          {t("suspendedOnly")}
        </label>

        <form onSubmit={applyTeacher} className="ml-auto flex gap-2">
          <input
            value={teacher}
            onChange={(e) => setTeacher(e.target.value)}
            placeholder={t("teacherSlug")}
            className="w-44 rounded-lg border border-input bg-transparent px-3 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <Button type="submit" size="sm" variant="outline">
            {t("filter")}
          </Button>
        </form>
      </div>

      {teacherQuery && (
        <p className="text-xs text-muted-foreground">
          {t("filteredTo")}{" "}
          <span className="font-medium text-foreground">{teacherQuery}</span> ·{" "}
          <button
            className="underline hover:text-foreground"
            onClick={() => {
              setTeacher("");
              setTeacherQuery("");
            }}
          >
            {t("clear")}
          </button>
        </p>
      )}

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoadCourses")}
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : rows.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
          {t("noCoursesMatch")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {rows.map((c) => (
            <CourseCard key={c.id} course={c} onChanged={reload} locale={locale} />
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

function CourseCard({
  course: c,
  onChanged,
  locale,
}: {
  course: AdminCourse;
  onChanged: () => void;
  locale: string;
}) {
  const t = useTranslations("admin");
  const tCourse = useTranslations("courses");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      onChanged();
    } catch (e) {
      setErr(e instanceof AdminError ? e.message : t("somethingWrong"));
      setBusy(false);
    }
  }

  return (
    <li
      className={cn(
        "rounded-2xl border bg-card p-5",
        c.suspended ? "border-dashed border-destructive/40" : "border-border",
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <div className="flex items-center gap-2">
            <span className="font-medium">{c.title}</span>
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-xs font-semibold",
                c.status === "published"
                  ? "bg-mint/20 text-mint-foreground"
                  : "bg-muted text-muted-foreground",
              )}
            >
              {tCourse(c.status)}
            </span>
            {c.archived && (
              <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-semibold text-muted-foreground">
                {t("archived")}
              </span>
            )}
            {c.suspended && (
              <span className="rounded-full bg-destructive/10 px-2 py-0.5 text-xs font-semibold text-destructive">
                {t("suspended")}
              </span>
            )}
          </div>
          <p className="mt-1 text-sm">
            <Link
              href={`/admin/teachers/${c.teacher.slug}`}
              className="font-medium text-primary hover:underline"
            >
              {c.teacher.displayName}
            </Link>{" "}
            <span className="text-muted-foreground">
              {t("createdOn", { price: formatMoney(c.price, locale), date: fmt(c.createdAt, locale) })}
              {c.suspended && c.suspendedAt ? ` ${t("suspendedOn", { date: fmt(c.suspendedAt, locale) })}` : ""}
            </span>
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap gap-2 border-t border-border pt-4">
        {c.suspended ? (
          <Button size="sm" disabled={busy} onClick={() => run(() => unsuspendCourse(c.id))}>
            {busy ? t("restoring") : t("restoreToStorefront")}
          </Button>
        ) : (
          <Button
            size="sm"
            variant="destructive"
            disabled={busy}
            onClick={() => run(() => suspendCourse(c.id))}
          >
            {busy ? t("suspending") : t("suspend")}
          </Button>
        )}
      </div>

      {err && (
        <p className="mt-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {err}
        </p>
      )}
    </li>
  );
}
