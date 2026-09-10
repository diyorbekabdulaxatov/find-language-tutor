"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import { BookingError, listBookings, type Booking } from "@/features/bookings/api";
import {
  formatDayLabel,
  formatTime,
  viewerTimezone,
} from "@/features/bookings/datetime";
import { BookingStatusBadge } from "./booking-status-badge";

type Role = "student" | "teacher";

export function BookingsList() {
  const [role, setRole] = useState<Role>("student");
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const viewerTz = viewerTimezone();

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
          setError(
            err instanceof BookingError
              ? err.message
              : "Could not load your bookings.",
          );
        }
      } finally {
        if (alive) setLoading(false);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [role]);

  const [now] = useState(() => Date.now());
  const upcoming = bookings.filter(
    (b) => new Date(b.startAt).getTime() >= now && b.status !== "cancelled",
  );
  const past = bookings.filter(
    (b) => new Date(b.startAt).getTime() < now || b.status === "cancelled",
  );

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="font-display text-3xl">Bookings</h1>

      <div className="mt-6 inline-flex rounded-lg border border-border p-0.5 text-sm">
        {(["student", "teacher"] as const).map((r) => (
          <button
            key={r}
            onClick={() => setRole(r)}
            className={cn(
              "rounded-md px-3 py-1.5 font-medium transition-colors",
              role === r
                ? "bg-secondary text-secondary-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {r === "student" ? "Lessons I'm taking" : "Lessons I'm teaching"}
          </button>
        ))}
      </div>

      {loading && (
        <div className="mt-8 space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-20 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      )}

      {error && (
        <p className="mt-8 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      {!loading && !error && bookings.length === 0 && (
        <p className="mt-8 rounded-xl bg-muted px-4 py-10 text-center text-sm text-muted-foreground">
          {role === "student"
            ? "No lessons booked yet. Find a teacher to get started."
            : "No lessons booked with you yet."}
        </p>
      )}

      {!loading && !error && bookings.length > 0 && (
        <div className="mt-8 flex flex-col gap-8">
          {upcoming.length > 0 && (
            <Group title="Upcoming">
              {upcoming.map((b) => (
                <BookingRow key={b.id} b={b} role={role} viewerTz={viewerTz} />
              ))}
            </Group>
          )}
          {past.length > 0 && (
            <Group title="Past & cancelled">
              {past.map((b) => (
                <BookingRow key={b.id} b={b} role={role} viewerTz={viewerTz} />
              ))}
            </Group>
          )}
        </div>
      )}
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
      <h2 className="mb-3 text-sm font-semibold text-muted-foreground">
        {title}
      </h2>
      <div className="flex flex-col gap-2">{children}</div>
    </section>
  );
}

function BookingRow({
  b,
  role,
  viewerTz,
}: {
  b: Booking;
  role: Role;
  viewerTz: string;
}) {
  const who =
    role === "student" ? b.teacher.displayName : b.student.displayName;
  return (
    <Link
      href={`/bookings/${b.id}`}
      className="flex items-center justify-between gap-4 rounded-xl border border-border bg-card p-4 transition-colors hover:bg-muted/50"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="truncate font-medium">{who}</span>
          {b.isTrial && (
            <span className="rounded-full bg-coral/10 px-2 py-0.5 text-xs font-semibold text-coral">
              Trial
            </span>
          )}
        </div>
        <div className="mt-0.5 text-sm text-muted-foreground">
          {formatDayLabel(b.startAt, viewerTz)} ·{" "}
          {formatTime(b.startAt, viewerTz)} · {b.durationMinutes} min
        </div>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <BookingStatusBadge status={b.status} />
        <span className="text-sm text-muted-foreground">
          {formatMoney(b.price)}
        </span>
      </div>
    </Link>
  );
}
