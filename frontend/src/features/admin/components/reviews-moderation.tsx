"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { intlLocale } from "@/lib/i18n";
import {
  AdminError,
  hideReview,
  listAdminReviews,
  removeReview,
  unhideReview,
  type AdminReview,
  type ReviewVisibility,
} from "@/features/admin/api";
import { Stars } from "@/features/reviews/components/star-rating";
import { Button } from "@/components/ui/button";

const PAGE_SIZE = 20;

const VISIBILITY: { value: ReviewVisibility; label: "visAll" | "visVisible" | "visHidden" }[] = [
  { value: "all", label: "visAll" },
  { value: "visible", label: "visVisible" },
  { value: "hidden", label: "visHidden" },
];

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

export function ReviewsModeration() {
  const [visibility, setVisibility] = useState<ReviewVisibility>("all");
  const [lowOnly, setLowOnly] = useState(false);
  const [teacher, setTeacher] = useState("");
  const [teacherQuery, setTeacherQuery] = useState("");
  const [page, setPage] = useState(1);

  const [rows, setRows] = useState<AdminReview[]>([]);
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
        const res = await listAdminReviews({
          visibility,
          teacher: teacherQuery || undefined,
          maxRating: lowOnly ? 3 : undefined,
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
  }, [visibility, lowOnly, teacherQuery, page, reloadKey]);

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
        {VISIBILITY.map((f) => (
          <button
            key={f.value}
            onClick={() => {
              setPage(1);
              setVisibility(f.value);
            }}
            className={cn(
              "rounded-lg border px-3 py-1.5 text-sm transition-colors",
              visibility === f.value
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
            checked={lowOnly}
            onChange={(e) => {
              setPage(1);
              setLowOnly(e.target.checked);
            }}
            className="accent-primary"
          />
          {t("lowRated")}
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
          {t("couldNotLoadReviews")}
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : rows.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-10 text-center text-sm text-muted-foreground">
          {t("noReviewsMatch")}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {rows.map((r) => (
            <ReviewCard key={r.id} review={r} onChanged={reload} locale={locale} />
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

function ReviewCard({
  review: r,
  onChanged,
  locale,
}: {
  review: AdminReview;
  onChanged: () => void;
  locale: string;
}) {
  const t = useTranslations("admin");
  const [busy, setBusy] = useState(false);
  const [confirmingRemove, setConfirmingRemove] = useState(false);
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
        r.hidden ? "border-dashed border-muted-foreground/40" : "border-border",
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <div className="flex items-center gap-2">
            <Stars value={r.rating} />
            {r.hidden && (
              <span className="rounded-full bg-muted px-2 py-0.5 text-xs font-semibold text-muted-foreground">
                {t("hidden")}
              </span>
            )}
            {r.sample && (
              <span className="rounded-full bg-accent px-2 py-0.5 text-xs font-semibold text-muted-foreground">
                {t("sample")}
              </span>
            )}
          </div>
          <p className="mt-1 text-sm">
            <Link
              href={`/admin/teachers/${r.teacher.slug}`}
              className="font-medium text-primary hover:underline"
            >
              {r.teacher.displayName}
            </Link>{" "}
            <span className="text-muted-foreground">
              {t("byOn", { name: r.studentName, date: fmt(r.createdAt, locale) })}
            </span>
          </p>
        </div>
      </div>

      <p className="mt-3 whitespace-pre-wrap text-sm">{r.comment || <span className="text-muted-foreground">{t("noComment")}</span>}</p>

      <div className="mt-4 flex flex-wrap gap-2 border-t border-border pt-4">
        {r.hidden ? (
          <Button size="sm" disabled={busy} onClick={() => run(() => unhideReview(r.id))}>
            {t("restore")}
          </Button>
        ) : (
          <Button
            size="sm"
            variant="outline"
            disabled={busy}
            onClick={() => run(() => hideReview(r.id))}
          >
            {t("hide")}
          </Button>
        )}

        {confirmingRemove ? (
          <>
            <Button
              size="sm"
              variant="destructive"
              disabled={busy}
              onClick={() => run(() => removeReview(r.id))}
            >
              {busy ? t("removing") : t("confirmRemove")}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => setConfirmingRemove(false)}
            >
              {t("cancel")}
            </Button>
          </>
        ) : (
          <Button
            size="sm"
            variant="ghost"
            className="text-destructive hover:text-destructive"
            disabled={busy}
            onClick={() => setConfirmingRemove(true)}
          >
            {t("removePermanently")}
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
