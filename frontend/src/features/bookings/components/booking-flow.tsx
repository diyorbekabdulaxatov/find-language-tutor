"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, CheckCircle2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import type { Money } from "@/types/teacher";
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
import { TeacherAvatar } from "@/features/teachers/components/teacher-avatar";

type Step = "pick" | "confirm" | "pay" | "done";

const STEPS: Step[] = ["pick", "confirm", "pay"];

/**
 * Udemy-style checkout: the step column on the left, a sticky order summary on
 * the right. The summary carries the price from the moment a slot is picked,
 * so the total never disappears between steps.
 */
export function BookingFlow({
  slug,
  teacherName,
  teacherAvatarUrl,
  teacherTimezone,
  listPrice,
  isTrial,
}: {
  slug: string;
  teacherName: string;
  teacherAvatarUrl: string;
  teacherTimezone: string;
  listPrice: Money;
  isTrial: boolean;
}) {
  const { status } = useAuth();
  const router = useRouter();
  const t = useTranslations("bookings");
  const locale = useLocale();

  const [step, setStep] = useState<Step>("pick");
  const [selection, setSelection] = useState<SlotSelection | null>(null);
  // the time highlighted in the picker, before "Continue" — the summary follows it
  const [preview, setPreview] = useState<SlotSelection | null>(null);
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
        setSelection(null);
        setPreview(null);
        setStep("pick");
        return;
      }
      setError(err instanceof BookingError ? err.message : t("couldNotCreate"));
    }
  }

  if (step === "done" && booking) {
    const paid = booking.status === "confirmed";
    return (
      <div className="mx-auto max-w-xl border border-border bg-card p-8 text-center shadow-card">
        <CheckCircle2 className="mx-auto size-10 text-primary" />
        <h2 className="mt-4 font-display text-2xl">
          {paid ? t("lessonBooked") : t("lessonReserved")}
        </h2>
        <p className="mx-auto mt-2 max-w-sm text-sm text-muted-foreground">
          {paid ? t("bookedBody", { name: teacherName }) : t("reservedBody", { name: teacherName })}
        </p>
        <p className="mt-4 text-sm font-bold">
          {formatFull(booking.startAt, viewerTz, locale)}{" "}
          <span className="font-normal text-muted-foreground">
            ({t("min", { count: booking.durationMinutes })} · {formatMoney(booking.price, locale)})
          </span>
        </p>
        <div className="mt-6 flex flex-col gap-2 sm:flex-row sm:justify-center">
          <Link
            href={`/bookings/${booking.id}`}
            className="flex h-12 items-center justify-center rounded-md bg-primary px-5 text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
          >
            {t("viewBooking")}
          </Link>
          <Link
            href="/bookings"
            className="flex h-12 items-center justify-center rounded-md border border-foreground bg-background px-5 text-base font-bold text-foreground transition-colors hover:bg-accent"
          >
            {t("allBookings")}
          </Link>
        </div>
      </div>
    );
  }

  // On the picker the summary mirrors what is highlighted there (nothing, on
  // a fresh mount); from the review step on it follows the committed choice.
  const shown = step === "pick" ? preview : selection;
  const price = shown?.slot.price ?? booking?.price ?? listPrice;
  const durationMinutes = shown?.durationMinutes ?? booking?.durationMinutes ?? null;
  const startAt = shown?.slot.startAt ?? booking?.startAt ?? null;

  return (
    <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-12">
      <div>
        <StepRail current={step} />

        {error && (
          <p role="alert" className="mt-6 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        {step === "pick" && (
          <div className="mt-6">
            <SlotPicker
              slug={slug}
              teacherTimezone={teacherTimezone}
              isTrial={isTrial}
              onPick={handlePick}
              onPreview={setPreview}
            />
          </div>
        )}

        {step === "confirm" && selection && (
          <div className="mt-6">
            <button
              type="button"
              onClick={() => {
                setPreview(null);
                setStep("pick");
              }}
              className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
            >
              <ArrowLeft className="size-4" /> {t("changeTime")}
            </button>

            <div className="mt-4 border border-border bg-card p-6">
              <h2 className="font-display text-xl">{t("confirmYourLesson")}</h2>
              <dl className="mt-4 divide-y divide-border border-y border-border text-sm">
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

              <p className="mt-4 bg-muted p-3 text-xs text-muted-foreground">{t("nextPay")}</p>

              <button
                type="button"
                onClick={handleConfirm}
                disabled={submitting}
                className="mt-5 flex h-12 w-full items-center justify-center rounded-md bg-primary text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8] disabled:opacity-60"
              >
                {submitting ? t("reserving") : t("continueToPayment")}
              </button>
            </div>
          </div>
        )}

        {step === "pay" && booking && (
          <div className="mt-6">
            <PaymentForm
              bookingId={booking.id}
              amount={booking.price}
              onPaid={(b) => {
                setBooking(b);
                setStep("done");
              }}
            />
            <button
              type="button"
              onClick={() => setStep("done")}
              className="mt-4 text-sm font-bold text-link hover:underline"
            >
              {t("skipForNow")}
            </button>
          </div>
        )}
      </div>

      <aside className="lg:sticky lg:top-6 lg:h-fit">
        <div className="border border-border bg-card p-5 shadow-card">
          <h2 className="font-display text-lg">{t("orderSummary")}</h2>

          <div className="mt-4 flex items-center gap-3 border-b border-border pb-4">
            <TeacherAvatar src={teacherAvatarUrl} name={teacherName} size={44} />
            <div className="min-w-0">
              <p className="truncate text-sm font-bold">{teacherName}</p>
              <p className="text-xs text-muted-foreground">
                {isTrial ? t("trialLesson") : t("lesson")}
              </p>
            </div>
          </div>

          <dl className="mt-4 space-y-2 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">{t("when")}</dt>
              <dd className="text-right font-bold">
                {startAt ? formatFull(startAt, viewerTz, locale) : t("pickTimeFirst")}
              </dd>
            </div>
            {durationMinutes != null && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted-foreground">{t("length")}</dt>
                <dd className="font-bold">{t("min", { count: durationMinutes })}</dd>
              </div>
            )}
          </dl>

          <div className="mt-4 flex items-baseline justify-between border-t border-border pt-4">
            <span className="text-sm font-bold">{t("total")}</span>
            <span className="font-display text-2xl">{formatMoney(price, locale)}</span>
          </div>

          <p className="mt-3 text-xs text-muted-foreground">{t("teacherPaidAfter")}</p>
        </div>
      </aside>
    </div>
  );
}

/** "1 Choose a time — 2 Review — 3 Payment", the done steps in ink. */
function StepRail({ current }: { current: Step }) {
  const t = useTranslations("bookings");
  const labels: Record<Step, string> = {
    pick: t("stepTime"),
    confirm: t("stepReview"),
    pay: t("stepPayment"),
    done: "",
  };
  const index = STEPS.indexOf(current);

  return (
    <ol className="flex items-center gap-3 border-b border-border pb-3 text-sm">
      {STEPS.map((s, i) => (
        <li key={s} className="flex items-center gap-3">
          {i > 0 && <span aria-hidden className="h-px w-4 bg-border sm:w-8" />}
          <span
            className={cn(
              "inline-flex items-center gap-2 whitespace-nowrap",
              i <= index ? "text-foreground" : "text-muted-foreground",
            )}
            aria-current={i === index ? "step" : undefined}
          >
            <span
              className={cn(
                "grid size-6 shrink-0 place-items-center rounded-full text-xs font-bold",
                i < index
                  ? "bg-foreground text-background"
                  : i === index
                    ? "bg-primary text-primary-foreground"
                    : "border border-border text-muted-foreground",
              )}
            >
              {i + 1}
            </span>
            <span className={cn("font-bold", i === index ? "" : "hidden sm:inline")}>
              {labels[s]}
            </span>
          </span>
        </li>
      ))}
    </ol>
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
    <div className="flex justify-between gap-4 py-2.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-bold text-foreground">{children}</dd>
    </div>
  );
}
