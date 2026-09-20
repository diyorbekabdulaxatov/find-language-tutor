"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, CheckCircle2, Info } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import type { Money } from "@/types/teacher";
import { useAuth } from "@/features/auth/auth-context";
import {
  BookingError,
  createBooking,
  getTrialEligibility,
  type Booking,
  type DURATION_OPTIONS,
  type TrialEligibility,
} from "@/features/bookings/api";
import { formatFull, viewerTimezone } from "@/features/bookings/datetime";
import {
  SlotPicker,
  type SlotSelection,
} from "@/features/bookings/components/slot-picker";
import { PaymentForm } from "@/features/bookings/components/payment-form";
import { TeacherAvatar } from "@/features/teachers/components/teacher-avatar";
import type { LessonType } from "@/features/teachers/api";

type Step = "lesson" | "pick" | "confirm" | "pay" | "done";

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
  lessonTypes = [],
  selectedLessonTypeId,
  selectedDuration,
}: {
  slug: string;
  teacherName: string;
  teacherAvatarUrl: string;
  teacherTimezone: string;
  listPrice: Money;
  isTrial: boolean;
  /** the teacher's offerings; empty for a profile that predates lesson types */
  lessonTypes?: LessonType[];
  /** preselected by the profile's lesson cards — skips the first step */
  selectedLessonTypeId?: string;
  selectedDuration?: number;
}) {
  const { status } = useAuth();
  const router = useRouter();
  const t = useTranslations("bookings");
  const locale = useLocale();

  // Choosing the lesson is step one — unless the profile already linked to one,
  // or the teacher has no offerings (the pre-lesson-type path).
  const [lessonTypeId, setLessonTypeId] = useState<string | undefined>(selectedLessonTypeId);
  const chosen = lessonTypes.find((lt) => lt.id === lessonTypeId);
  // "Choose another lesson" from a blocked trial reopens the lesson step even
  // though the profile linked straight to an offering.
  const [lessonStepReopened, setLessonStepReopened] = useState(false);
  const needsLessonStep = lessonTypes.length > 0 && (!selectedLessonTypeId || lessonStepReopened);
  const steps: Step[] = needsLessonStep
    ? ["lesson", "pick", "confirm", "pay"]
    : ["pick", "confirm", "pay"];

  const [step, setStep] = useState<Step>(needsLessonStep ? "lesson" : "pick");

  // One trial per student per teacher. Asked once the session is known; until
  // then (and for a signed-out visitor) the trial card stays available and the
  // server is the authority at create time.
  const [trialEligibility, setTrialEligibility] = useState<TrialEligibility | null>(null);
  useEffect(() => {
    if (status !== "authenticated") return;
    let cancelled = false;
    async function load() {
      try {
        const e = await getTrialEligibility(slug);
        if (!cancelled) setTrialEligibility(e);
      } catch {
        // best effort: the create call still enforces the rule
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [slug, status]);
  const heldTrialId =
    trialEligibility && !trialEligibility.eligible && trialEligibility.reason === "already_booked"
      ? trialEligibility.bookingId
      : null;
  // The picked (or pre-linked) lesson is a trial the student has already used.
  const trialBlocked = heldTrialId != null && (chosen ? chosen.isTrial : isTrial);

  function chooseAnotherLesson() {
    setPreview(null);
    setSelection(null);
    setError(null);
    if (lessonTypes.length > 0) {
      setLessonTypeId(undefined);
      setLessonStepReopened(true);
      setStep("lesson");
    } else {
      router.push(`/teachers/${slug}/book`);
    }
  }
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
        lessonTypeId,
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
      if (err instanceof BookingError && err.code === "trial_already_booked") {
        // A trial booked in another tab, or before the eligibility check
        // landed: surface the rule where the lesson is chosen.
        setTrialEligibility({ eligible: false, reason: "already_booked", bookingId: "" });
        setSelection(null);
        setPreview(null);
        if (lessonTypes.length > 0) {
          setLessonTypeId(undefined);
          setLessonStepReopened(true);
          setStep("lesson");
        } else {
          setStep("pick");
        }
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
        <StepRail current={step} steps={steps} />

        {error && (
          <p role="alert" className="mt-6 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        {step === "lesson" && (
          <div className="mt-6">
            {heldTrialId != null && (
              <TrialUsedNotice
                teacherName={teacherName}
                bookingId={heldTrialId}
                className="mb-6"
              />
            )}
            <LessonPicker
              lessonTypes={lessonTypes}
              locale={locale}
              disabledTrial={heldTrialId != null}
              onPick={(id) => {
                setLessonTypeId(id);
                setPreview(null);
                setStep("pick");
              }}
            />
          </div>
        )}

        {step === "pick" && trialBlocked && (
          <div className="mt-6">
            <TrialUsedNotice teacherName={teacherName} bookingId={heldTrialId} />
            <button
              type="button"
              onClick={chooseAnotherLesson}
              className="mt-4 flex h-12 items-center justify-center rounded-md bg-primary px-5 text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
            >
              {t("trialUsedChoose")}
            </button>
          </div>
        )}

        {step === "pick" && !trialBlocked && (
          <div className="mt-6">
            {needsLessonStep && (
              <button
                type="button"
                onClick={() => {
                  setPreview(null);
                  setStep("lesson");
                }}
                className="mb-4 inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
              >
                <ArrowLeft className="size-4" /> {t("changeLesson")}
              </button>
            )}
            <SlotPicker
              slug={slug}
              teacherTimezone={teacherTimezone}
              isTrial={isTrial}
              onPick={handlePick}
              onPreview={setPreview}
              lessonTypeId={lessonTypeId}
              durations={chosen?.prices.map((p) => p.durationMinutes)}
              initialDuration={
                selectedDuration && chosen?.prices.some((p) => p.durationMinutes === selectedDuration)
                  ? (selectedDuration as (typeof DURATION_OPTIONS)[number])
                  : undefined
              }
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
                {chosen && <Row label={t("lesson")}>{chosen.title}</Row>}
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
                {chosen
                  ? chosen.title
                  : isTrial && !lessonStepReopened
                    ? t("trialLesson")
                    : t("lesson")}
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
function StepRail({ current, steps }: { current: Step; steps: Step[] }) {
  const t = useTranslations("bookings");
  const labels: Record<Step, string> = {
    lesson: t("stepLesson"),
    pick: t("stepTime"),
    confirm: t("stepReview"),
    pay: t("stepPayment"),
    done: "",
  };
  const index = steps.indexOf(current);

  return (
    <ol className="flex items-center gap-3 border-b border-border pb-3 text-sm">
      {steps.map((s, i) => (
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

/**
 * Step one when the teacher lists offerings: the same cards as the profile, as
 * radio-style buttons. Picking one moves straight to the calendar, with that
 * offering's lengths and prices.
 */
function LessonPicker({
  lessonTypes,
  locale,
  disabledTrial = false,
  onPick,
}: {
  lessonTypes: LessonType[];
  locale: string;
  /** the student has used their trial with this teacher — the trial card is shown, not pickable */
  disabledTrial?: boolean;
  onPick: (id: string) => void;
}) {
  const t = useTranslations("bookings");
  return (
    <div className="flex flex-col gap-3">
      <h2 className="font-display text-xl">{t("chooseLesson")}</h2>
      <ul className="flex flex-col gap-3">
        {lessonTypes.map((lt) => {
          const blocked = lt.isTrial && disabledTrial;
          return (
          <li key={lt.id}>
            <button
              type="button"
              onClick={() => onPick(lt.id)}
              disabled={blocked}
              aria-disabled={blocked || undefined}
              className={cn(
                "w-full border border-border bg-card p-4 text-left transition-colors",
                blocked
                  ? "cursor-not-allowed opacity-60"
                  : "hover:border-foreground hover:bg-accent",
              )}
            >
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <span className="font-bold">
                  {lt.title}
                  {lt.isTrial && (
                    <span className="ml-2 rounded-sm bg-accent px-1.5 py-0.5 text-xs font-bold text-accent-foreground">
                      {blocked ? t("trialUsedBadge") : t("trial")}
                    </span>
                  )}
                </span>
                <span className="text-sm text-muted-foreground">
                  {t("fromPrice", { price: formatMoney(lt.from, locale) })}
                </span>
              </div>
              {lt.description && (
                <p className="mt-1 text-sm text-muted-foreground">{lt.description}</p>
              )}
              <p className="mt-2 text-xs text-muted-foreground">
                {lt.prices.map((p) => t("min", { count: p.durationMinutes })).join(" · ")}
              </p>
            </button>
          </li>
          );
        })}
      </ul>
    </div>
  );
}

/** "You've already had your trial with X" — one trial per student per teacher. */
function TrialUsedNotice({
  teacherName,
  bookingId,
  className,
}: {
  teacherName: string;
  /** "" when the rule surfaced from the create call and the id is unknown */
  bookingId: string | null;
  className?: string;
}) {
  const t = useTranslations("bookings");
  return (
    <div
      role="status"
      className={cn("flex gap-3 border border-border bg-muted p-4 text-sm", className)}
    >
      <Info className="mt-0.5 size-4 shrink-0 text-link" aria-hidden />
      <div>
        <p className="font-bold">{t("trialUsedTitle", { name: teacherName })}</p>
        <p className="mt-1 text-muted-foreground">{t("trialUsedBody", { name: teacherName })}</p>
        {bookingId && (
          <Link
            href={`/bookings/${bookingId}`}
            className="mt-2 inline-block font-bold text-link hover:underline"
          >
            {t("trialUsedView")}
          </Link>
        )}
      </div>
    </div>
  );
}
