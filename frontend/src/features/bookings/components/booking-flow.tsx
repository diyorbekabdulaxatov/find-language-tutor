"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
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
import { PaymentForm } from "@/features/bookings/components/payment-form";
import { Button } from "@/components/ui/button";

type Step = "pick" | "confirm" | "pay" | "done";

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
  const t = useTranslations("bookings");
  const tBook = useTranslations("bookPage");
  const locale = useLocale();

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
      setSubmitting(false);
      setStep("pay");
    } catch (err) {
      setSubmitting(false);
      if (
        err instanceof BookingError &&
        (err.code === "slot_taken" || err.code === "slot_unavailable")
      ) {
        setError(t("slotGone"));
        setStep("pick");
        return;
      }
      setError(err instanceof BookingError ? err.message : t("couldNotCreate"));
    }
  }

  if (step === "done" && booking) {
    const paid = booking.status === "confirmed";
    return (
      <div className="flex flex-col items-center gap-4 rounded-2xl border border-border bg-card p-8 text-center">
        <CheckCircle2 className="size-10 text-primary" />
        <h2 className="font-display text-2xl">{paid ? t("lessonBooked") : t("lessonReserved")}</h2>
        <p className="max-w-sm text-sm text-muted-foreground">
          {paid ? t("bookedBody", { name: teacherName }) : t("reservedBody", { name: teacherName })}
        </p>
        <p className="text-sm">
          {formatFull(booking.startAt, viewerTz, locale)}{" "}
          <span className="text-muted-foreground">
            ({t("min", { count: booking.durationMinutes })} · {formatMoney(booking.price, locale)})
          </span>
        </p>
        <div className="flex gap-3">
          <Button asChild>
            <Link href={`/bookings/${booking.id}`}>{t("viewBooking")}</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/bookings">{t("allBookings")}</Link>
          </Button>
        </div>
      </div>
    );
  }

  if (step === "pay" && booking) {
    return (
      <div className="flex flex-col gap-5">
        <button
          type="button"
          onClick={() => setStep("done")}
          className="inline-flex items-center gap-1.5 self-start text-sm font-medium text-muted-foreground hover:text-foreground"
        >
          {t("skipForNow")}
        </button>
        <PaymentForm
          bookingId={booking.id}
          amount={booking.price}
          onPaid={(b) => {
            setBooking(b);
            setStep("done");
          }}
        />
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
          <ArrowLeft className="size-4" /> {t("changeTime")}
        </button>

        <div className="rounded-2xl border border-border bg-card p-6">
          <h2 className="font-display text-xl">{t("confirmYourLesson")}</h2>
          <dl className="mt-4 flex flex-col gap-3 text-sm">
            <Row label={t("teacher")}>{teacherName}</Row>
            <Row label={t("when")}>{formatFull(selection.slot.startAt, viewerTz, locale)}</Row>
            <Row label={t("yourTimezone")}>{viewerTz}</Row>
            {viewerTz !== teacherTimezone && (
              <Row label={t("teachersTime")}>
                {formatFull(selection.slot.startAt, teacherTimezone, locale)}
              </Row>
            )}
            <Row label={t("length")}>
              {selection.isTrial
                ? t("trialLength", { count: selection.durationMinutes })
                : t("min", { count: selection.durationMinutes })}
            </Row>
            <Row label={t("price")}>{formatMoney(selection.slot.price, locale)}</Row>
          </dl>

          <p className="mt-4 rounded-lg bg-primary/8 p-3 text-xs text-muted-foreground">{t("nextPay")}</p>

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
            {submitting ? t("reserving") : t("continueToPayment")}
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <CalendarCheck className="size-4" />
        {t("bookWith", { title: tBook(isTrial ? "bookTrial" : "bookLesson"), name: teacherName })}
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
