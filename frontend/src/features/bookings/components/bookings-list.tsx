"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { BookingError, listBookings, type Booking } from "@/features/bookings/api";
import {
  formatDayLabel,
  formatTime,
  viewerTimezone,
} from "@/features/bookings/datetime";
import { TeacherAvatar } from "@/features/teachers/components/teacher-avatar";
import { BookingStatusBadge } from "./booking-status-badge";

type Role = "student" | "teacher";

export function BookingsList() {
  const [role, setRole] = useState<Role>("student");
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const viewerTz = viewerTimezone();
  const t = useTranslations("bookings");
  const locale = useLocale();

  useEffect(() => {
    let alive = true;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const b = await listBookings(role);
        if (alive) setBookings(b);
      } catch (err) {
        if (!alive) return;
        // teacher view 404s when the account owns no profile — treat as empty
        if (err instanceof BookingError && err.status === 404) {
          setBookings([]);
        } else {
          setError(err instanceof BookingError ? err.message : t("couldNotLoad"));
        }
      } finally {
        if (alive) setLoading(false);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [role, t]);

  const [now] = useState(() => Date.now());
  const upcoming = bookings.filter(
    (b) => new Date(b.startAt).getTime() >= now && b.status !== "cancelled",
  );
  const past = bookings.filter(
    (b) => new Date(b.startAt).getTime() < now || b.status === "cancelled",
  );

  return (
    <div>
      {/* The dark title band with the role tabs on its bottom edge — the same
          shape as My learning, so the two student surfaces read as siblings. */}
      <div className="bg-ink text-ink-foreground">
        <div className="mx-auto max-w-[1340px] px-4 pt-10 sm:px-6">
          <h1 className="font-display text-3xl sm:text-[2.5rem]">{t("title")}</h1>
          <p className="mt-2 text-sm text-white/80">{t("listIntro")}</p>
          <div role="tablist" className="mt-6 flex gap-6 text-base font-bold">
            {(["student", "teacher"] as const).map((r) => (
              <button
                key={r}
                role="tab"
                aria-selected={role === r}
                onClick={() => setRole(r)}
                className={cn(
                  "-mb-px border-b-4 pb-2 transition-colors",
                  role === r
                    ? "border-white text-white"
                    : "border-transparent text-white/70 hover:text-white",
                )}
              >
                {r === "student" ? t("lessonsTaking") : t("lessonsTeaching")}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6">
      {loading && (
        <div className="space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-20 animate-pulse bg-muted" />
          ))}
        </div>
      )}

      {error && (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      {!loading && !error && bookings.length === 0 && (
        <div className="border border-border px-6 py-16 text-center">
          <p className="text-xl font-bold">
            {role === "student" ? t("noneStudent") : t("noneTeacher")}
          </p>
          {role === "student" && (
            <Link
              href="/teachers"
              className="mt-4 inline-flex h-12 items-center rounded-md bg-primary px-4 text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
            >
              {t("findTeacher")}
            </Link>
          )}
        </div>
      )}

      {!loading && !error && bookings.length > 0 && (
        <div className="flex flex-col gap-8">
          {upcoming.length > 0 && (
            <Group title={t("upcoming")}>
              {upcoming.map((b) => (
                <BookingRow key={b.id} b={b} role={role} viewerTz={viewerTz} locale={locale} />
              ))}
            </Group>
          )}
          {past.length > 0 && (
            <Group title={t("pastCancelled")}>
              {past.map((b) => (
                <BookingRow key={b.id} b={b} role={role} viewerTz={viewerTz} locale={locale} />
              ))}
            </Group>
          )}
        </div>
      )}
      </div>
    </div>
  );
}

function Group({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h2 className="mb-3 font-display text-lg">{title}</h2>
      <div className="border-t border-border">{children}</div>
    </section>
  );
}

function BookingRow({
  b,
  role,
  viewerTz,
  locale,
}: {
  b: Booking;
  role: Role;
  viewerTz: string;
  locale: string;
}) {
  const t = useTranslations("bookings");
  const who =
    role === "student" ? b.teacher.displayName : b.student.displayName;
  return (
    <Link
      href={`/bookings/${b.id}`}
      className="flex items-center justify-between gap-4 border-b border-border px-1 py-4 transition-colors hover:bg-muted"
    >
      <div className="flex min-w-0 items-center gap-3">
        <TeacherAvatar
          src={role === "student" ? b.teacher.avatarUrl : ""}
          name={who}
          size={48}
        />
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate text-base font-bold">{who}</span>
            {b.isTrial && (
              <span className="rounded-sm bg-accent px-1.5 py-0.5 text-xs font-bold text-accent-foreground">
                {t("trial")}
              </span>
            )}
          </div>
          <div className="mt-0.5 text-sm text-muted-foreground">
            {formatDayLabel(b.startAt, viewerTz, locale)} ·{" "}
            {formatTime(b.startAt, viewerTz, locale)} · {t("min", { count: b.durationMinutes })}
          </div>
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <BookingStatusBadge status={b.status} />
        <span className="text-sm font-bold">{formatMoney(b.price, locale)}</span>
      </div>
    </Link>
  );
}
