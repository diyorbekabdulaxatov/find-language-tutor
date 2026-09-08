"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, Video } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { useAuth } from "@/features/auth/auth-context";
import {
  BookingError,
  cancelBooking,
  confirmBooking,
  getBooking,
  type Booking,
} from "@/features/bookings/api";
import { formatFull, viewerTimezone } from "@/features/bookings/datetime";
import { BookingStatusBadge } from "./booking-status-badge";
import { Button } from "@/components/ui/button";

export function BookingDetail({ id }: { id: string }) {
  const { user } = useAuth();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState<"confirm" | "cancel" | null>(null);
  const [now] = useState(() => Date.now());
  const viewerTz = viewerTimezone();

  useEffect(() => {
    let alive = true;
    getBooking(id)
      .then((b) => {
        if (!alive) return;
        setBooking(b);
        setState("ready");
      })
      .catch((err) => {
        if (!alive) return;
        setErrorMsg(
          err instanceof BookingError
            ? err.message
            : "Could not load that booking.",
        );
        setState("error");
      });
    return () => {
      alive = false;
    };
  }, [id]);

  async function run(
    action: "confirm" | "cancel",
    fn: () => Promise<Booking>,
  ) {
    setBusy(action);
    setErrorMsg(null);
    try {
      setBooking(await fn());
    } catch (err) {
      setErrorMsg(
        err instanceof BookingError
          ? err.message
          : "Something went wrong. Please try again.",
      );
    } finally {
      setBusy(null);
    }
  }

  if (state === "loading") {
    return (
      <div className="mx-auto max-w-xl px-4 py-12 sm:px-6">
        <div className="h-72 animate-pulse rounded-2xl bg-muted" />
      </div>
    );
  }

  if (state === "error" || !booking) {
    return (
      <div className="mx-auto max-w-xl px-4 py-12 sm:px-6">
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMsg ?? "Booking not found."}
        </p>
        <Link
          href="/bookings"
          className="mt-4 inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> All bookings
        </Link>
      </div>
    );
  }

  const isStudent = user?.id === booking.student.id;
  const canCancel =
    booking.status === "pending_payment" || booking.status === "confirmed";
  const canConfirm = isStudent && booking.status === "pending_payment";
  const startsSoon =
    new Date(booking.startAt).getTime() - now < 10 * 60_000 &&
    new Date(booking.endAt).getTime() > now;

  return (
    <div className="mx-auto max-w-xl px-4 py-10 sm:px-6">
      <Link
        href="/bookings"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="size-4" /> All bookings
      </Link>

      <div className="mt-4 rounded-2xl border border-border bg-card p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="font-display text-2xl">
              {isStudent
                ? `Lesson with ${booking.teacher.displayName}`
                : `Lesson with ${booking.student.displayName}`}
            </h1>
            {booking.isTrial && (
              <span className="mt-1 inline-block rounded-full bg-coral/10 px-2 py-0.5 text-xs font-semibold text-coral">
                Trial lesson
              </span>
            )}
          </div>
          <BookingStatusBadge status={booking.status} />
        </div>

        <dl className="mt-5 flex flex-col gap-3 border-t border-border pt-5 text-sm">
          <Row label="When (your time)">
            {formatFull(booking.startAt, viewerTz)}
          </Row>
          {viewerTz !== booking.teacher.timezone && (
            <Row label="Teacher's time">
              {formatFull(booking.startAt, booking.teacher.timezone)}
            </Row>
          )}
          <Row label="Length">{booking.durationMinutes} min</Row>
          <Row label="Price">{formatMoney(booking.price)}</Row>
          {booking.status === "cancelled" && booking.cancellationReason && (
            <Row label="Cancellation reason">{booking.cancellationReason}</Row>
          )}
        </dl>

        {booking.status === "confirmed" && (
          <div className="mt-5 rounded-xl bg-primary/8 p-4">
            <div className="flex items-center gap-2 text-sm font-medium">
              <Video className="size-4" /> Video call
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              {startsSoon
                ? "The meeting link will appear here — joining opens near the start time."
                : "A join button will appear here shortly before the lesson."}
            </p>
          </div>
        )}

        {errorMsg && (
          <p className="mt-4 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {errorMsg}
          </p>
        )}

        {(canConfirm || canCancel) && (
          <div className="mt-6 flex flex-wrap gap-3">
            {canConfirm && (
              <Button
                onClick={() => run("confirm", () => confirmBooking(booking.id))}
                disabled={busy !== null}
              >
                {busy === "confirm" ? "Confirming…" : "Confirm (skip payment)"}
              </Button>
            )}
            {canCancel && (
              <Button
                variant="destructive"
                onClick={() => run("cancel", () => cancelBooking(booking.id))}
                disabled={busy !== null}
              >
                {busy === "cancel" ? "Cancelling…" : "Cancel lesson"}
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function Row({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium text-foreground">{children}</dd>
    </div>
  );
}
