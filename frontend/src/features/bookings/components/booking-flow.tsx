"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, CalendarCheck, CheckCircle2 } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { useAuth } from "@/features/auth/auth-context";
import {
  BookingError,
  createBooking,
  type Booking,
} from "@/features/bookings/api";
import { formatFull, viewerTimezone } from "@/features/bookings/datetime";
import {
  SlotPicker,
  type SlotSelection,
} from "@/features/bookings/components/slot-picker";
import { Button } from "@/components/ui/button";

type Step = "pick" | "confirm" | "done";

export function BookingFlow({
  slug,
  teacherName,
  teacherTimezone,
  isTrial,
}: {
  slug: string;
  teacherName: string;
  teacherTimezone: string;
  isTrial: boolean;
}) {
  const { status } = useAuth();
  const router = useRouter();

  const [step, setStep] = useState<Step>("pick");
  const [selection, setSelection] = useState<SlotSelection | null>(null);
  const [booking, setBooking] = useState<Booking | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const viewerTz = viewerTimezone();

  function handlePick(sel: SlotSelection) {
    setSelection(sel);
    setError(null);
    if (status !== "authenticated") {
      router.push(
        `/login?next=${encodeURIComponent(`/teachers/${slug}/book${isTrial ? "?trial=1" : ""}`)}`,
      );
      return;
    }
    setStep("confirm");
  }

  async function handleConfirm() {
    if (!selection) return;
    setSubmitting(true);
    setError(null);
    try {
      const b = await createBooking({
        teacherSlug: slug,
        startAt: selection.slot.startAt,
        durationMinutes: selection.durationMinutes,
        isTrial: selection.isTrial,
      });
      setBooking(b);
      setStep("done");
    } catch (err) {
      setSubmitting(false);
      if (
        err instanceof BookingError &&
        (err.code === "slot_taken" || err.code === "slot_unavailable")
      ) {
        setError(
          "That time isn't available any more — someone may have just booked it. Pick another.",
        );
        setStep("pick");
        return;
      }
      setError(
        err instanceof BookingError
          ? err.message
          : "Could not create the booking. Please try again.",
      );
    }
  }

  if (step === "done" && booking) {
    return (
      <div className="flex flex-col items-center gap-4 rounded-2xl border border-border bg-card p-8 text-center">
        <CheckCircle2 className="size-10 text-primary" />
        <h2 className="font-display text-2xl">Lesson requested</h2>
        <p className="max-w-sm text-sm text-muted-foreground">
          Your lesson with {teacherName} is held as{" "}
          <span className="font-medium text-foreground">pending payment</span>.
          Payment comes next — for now you can see it in your bookings.
        </p>
        <p className="text-sm">
          {formatFull(booking.startAt, viewerTz)}{" "}
          <span className="text-muted-foreground">
            ({booking.durationMinutes} min · {formatMoney(booking.price)})
          </span>
        </p>
        <div className="flex gap-3">
          <Button asChild>
            <Link href={`/bookings/${booking.id}`}>View booking</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/bookings">All bookings</Link>
          </Button>
        </div>
      </div>
    );
  }

  if (step === "confirm" && selection) {
    return (
      <div className="flex flex-col gap-5">
        <button
          type="button"
          onClick={() => setStep("pick")}
          className="inline-flex items-center gap-1.5 self-start text-sm font-medium text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" /> Change time
        </button>

        <div className="rounded-2xl border border-border bg-card p-6">
          <h2 className="font-display text-xl">Confirm your lesson</h2>
          <dl className="mt-4 flex flex-col gap-3 text-sm">
            <Row label="Teacher">{teacherName}</Row>
            <Row label="When">{formatFull(selection.slot.startAt, viewerTz)}</Row>
            <Row label="Your timezone">{viewerTz}</Row>
            {viewerTz !== teacherTimezone && (
              <Row label="Teacher's time">
                {formatFull(selection.slot.startAt, teacherTimezone)}
              </Row>
            )}
            <Row label="Length">
              {selection.isTrial
                ? `Trial lesson (${selection.durationMinutes} min)`
                : `${selection.durationMinutes} min`}
            </Row>
            <Row label="Price">{formatMoney(selection.slot.price)}</Row>
          </dl>

          <p className="mt-4 rounded-lg bg-primary/8 p-3 text-xs text-muted-foreground">
            You won&apos;t be charged yet. The booking is held as pending until
            payment (coming soon).
          </p>

          {error && (
            <p className="mt-4 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </p>
          )}

          <Button
            className="mt-5 w-full"
            size="lg"
            onClick={handleConfirm}
            disabled={submitting}
          >
            {submitting ? "Booking…" : "Confirm booking"}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <CalendarCheck className="size-4" />
        {isTrial ? "Book a trial lesson" : "Book a lesson"} with {teacherName}
      </div>
      {error && (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}
      <SlotPicker
        slug={slug}
        teacherTimezone={teacherTimezone}
        isTrial={isTrial}
        onPick={handlePick}
      />
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
