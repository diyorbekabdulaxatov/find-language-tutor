"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { useAuth } from "@/features/auth/auth-context";
import {
  BookingError,
  cancelBooking,
  completeBooking,
  getBooking,
  rescheduleBooking,
  reportNoShow,
  type Duration,
  setMeetingLink,
  type Booking,
} from "@/features/bookings/api";
import { formatFull, viewerTimezone } from "@/features/bookings/datetime";
import {
  BookingReview,
  ReviewPrompt,
} from "@/features/reviews/components/review-prompt";
import { BookingStatusBadge } from "./booking-status-badge";
import { PaymentForm } from "./payment-form";
import { LessonJoinCard } from "./lesson-join-card";
import { SlotPicker, type SlotSelection } from "./slot-picker";
import { DisputePanel } from "./dispute-panel";
import { LessonResourcesPanel } from "./lesson-resources-panel";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FREE_CANCEL_HOURS } from "@/lib/policy";

export function BookingDetail({ id }: { id: string }) {
  const { user } = useAuth();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState<
    "complete" | "cancel" | "move" | "no_show_student" | "no_show_teacher" | "link" | null
  >(null);
  const [payOpen, setPayOpen] = useState(false);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [moveOpen, setMoveOpen] = useState(false);
  const [moveTo, setMoveTo] = useState<SlotSelection | null>(null);
  const [linkDraft, setLinkDraft] = useState("");
  const [now] = useState(() => Date.now());
  const viewerTz = viewerTimezone();
  const t = useTranslations("bookings");
  const tPay = useTranslations("paymentStatus");
  const tCommon = useTranslations("common");
  const locale = useLocale();

  useEffect(() => {
    let alive = true;
    getBooking(id)
      .then((b) => {
        if (!alive) return;
        setBooking(b);
        setLinkDraft(b.meetingUrl);
        setState("ready");
      })
      .catch((err) => {
        if (!alive) return;
        setErrorMsg(err instanceof BookingError ? err.message : t("couldNotLoadBooking"));
        setState("error");
      });
    return () => {
      alive = false;
    };
  }, [id, t]);

  async function refetch() {
    const fresh = await getBooking(id);
    setBooking(fresh);
    setLinkDraft(fresh.meetingUrl);
  }

  async function run(
    action: NonNullable<typeof busy>,
    fn: () => Promise<Booking>,
  ) {
    setBusy(action);
    setErrorMsg(null);
    try {
      await fn();
      // Re-fetch: some responses (cancel) omit the embedded payment.
      const fresh = await getBooking(id);
      setBooking(fresh);
      setLinkDraft(fresh.meetingUrl);
    } catch (err) {
      setErrorMsg(err instanceof BookingError ? err.message : tCommon("somethingWrong"));
    } finally {
      setBusy(null);
    }
  }

  if (state === "loading") {
    return (
      <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6">
        <div className="h-72 animate-pulse bg-muted" />
      </div>
    );
  }

  if (state === "error" || !booking) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6">
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errorMsg ?? t("notFound")}
        </p>
        <Link
          href="/bookings"
          className="mt-4 inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
        >
          <ArrowLeft className="size-4" /> {t("allBookings")}
        </Link>
      </div>
    );
  }

  const isStudent = user?.id === booking.student.id;
  const isTeacher = !isStudent; // only participants reach this page
  const ended = new Date(booking.endAt).getTime() < now;

  const started = new Date(booking.startAt).getTime() < now;
  const canPay = isStudent && booking.status === "pending_payment";
  const canComplete = isTeacher && booking.status === "confirmed" && ended;
  const canNoShow = isTeacher && booking.status === "confirmed" && started;
  const canCancel =
    booking.status === "pending_payment" || booking.status === "confirmed";
  // The policy only bites a paid booking cancelled by the student; the teacher
  // always refunds and an unpaid booking is simply dropped.
  const policy = booking.cancellationPolicy;
  const forfeits = isStudent && booking.status === "confirmed" && policy?.late === true;

  async function confirmMove() {
    if (!moveTo) return;
    setBusy("move");
    setErrorMsg(null);
    try {
      await rescheduleBooking(booking!.id, moveTo.slot.startAt);
      setMoveOpen(false);
      setMoveTo(null);
      await refetch();
    } catch (err) {
      if (
        err instanceof BookingError &&
        (err.code === "slot_taken" || err.code === "slot_unavailable")
      ) {
        setMoveTo(null); // pick again; the picker reloads on remount
        setErrorMsg(t("slotGone"));
      } else {
        setErrorMsg(err instanceof BookingError ? err.message : tCommon("somethingWrong"));
        if (err instanceof BookingError) await refetch().catch(() => undefined);
      }
    } finally {
      setBusy(null);
    }
  }

  async function confirmCancel() {
    setBusy("cancel");
    setErrorMsg(null);
    try {
      await cancelBooking(booking!.id, {
        reason: cancelReason.trim() || undefined,
        acknowledgeForfeit: forfeits,
      });
      setCancelOpen(false);
      setCancelReason("");
      await refetch();
    } catch (err) {
      if (err instanceof BookingError && err.code === "late_cancellation") {
        // The deadline passed while the panel was open: reload so the
        // warning and the button say what will really happen.
        await refetch().catch(() => undefined);
      }
      setErrorMsg(err instanceof BookingError ? err.message : tCommon("somethingWrong"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
      <Link
        href="/bookings"
        className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
      >
        <ArrowLeft className="size-4" /> {t("allBookings")}
      </Link>

      <div className="mt-4 border border-border bg-card p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="font-display text-2xl">
              {t("lessonWith", {
                name: isStudent ? booking.teacher.displayName : booking.student.displayName,
              })}
            </h1>
            {booking.isTrial && (
              <span className="mt-1 inline-block rounded-sm bg-accent px-2 py-0.5 text-xs font-bold text-accent-foreground">
                {t("trialLesson")}
              </span>
            )}
          </div>
          <BookingStatusBadge status={booking.status} />
        </div>

        <dl className="mt-5 divide-y divide-border border-t border-border text-sm">
          <Row label={t("whenYourTime")}>
            {formatFull(booking.startAt, viewerTz, locale)}
          </Row>
          {viewerTz !== booking.teacher.timezone && (
            <Row label={t("teachersTime")}>
              {formatFull(booking.startAt, booking.teacher.timezone, locale)}
            </Row>
          )}
          <Row label={t("length")}>{t("min", { count: booking.durationMinutes })}</Row>
          <Row label={t("price")}>{formatMoney(booking.price, locale)}</Row>
          {booking.payment && (
            <Row label={t("payment")}>{tPay(booking.payment.status)}</Row>
          )}
          {booking.noShowParty && (
            <Row label={t("noShow")}>
              {booking.noShowParty === "student" ? t("studentNoShow") : t("teacherNoShow")}
            </Row>
          )}
          {booking.status === "cancelled" && booking.cancellationReason && (
            <Row label={t("cancellationReason")}>{booking.cancellationReason}</Row>
          )}
          {booking.status === "cancelled" && booking.cancellationOutcome && (
            <Row label={t("cancellationOutcome")}>
              {t(`outcome.${booking.cancellationOutcome}`)}
            </Row>
          )}
          {booking.rescheduledFrom && booking.status !== "cancelled" && (
            <Row label={t("movedFrom")}>
              {formatFull(booking.rescheduledFrom, viewerTz, locale)}
            </Row>
          )}
          {canCancel && policy && booking.status === "confirmed" && (
            <Row label={t("cancellation")}>
              {policy.late
                ? isStudent
                  ? t("policyLateStudent")
                  : t("policyLateTeacher")
                : t("policyFreeUntil", {
                    when: formatFull(policy.freeCancelUntil, viewerTz, locale),
                  })}
            </Row>
          )}
        </dl>

        {booking.status === "confirmed" && (
          <LessonJoinCard
            startAt={booking.startAt}
            endAt={booking.endAt}
            meetingUrl={booking.meetingUrl}
            isTeacher={isTeacher}
          />
        )}

        {isTeacher &&
          (booking.status === "confirmed" || booking.status === "pending_payment") && (
            <form
              className="mt-4 flex flex-col gap-2"
              onSubmit={(e) => {
                e.preventDefault();
                run("link", () => setMeetingLink(booking.id, linkDraft.trim()));
              }}
            >
              <label htmlFor="meeting-link" className="text-sm font-bold">
                {t("meetingLink")}
              </label>
              <div className="flex gap-2">
                <Input
                  id="meeting-link"
                  type="url"
                  placeholder="https://meet.example.com/your-room"
                  value={linkDraft}
                  onChange={(e) => setLinkDraft(e.target.value)}
                />
                <Button
                  type="submit"
                  variant="outline"
                  disabled={busy !== null || linkDraft.trim() === booking.meetingUrl}
                >
                  {busy === "link" ? t("saving") : t("save")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">{t("meetingLinkHint")}</p>
            </form>
          )}

        {errorMsg && (
          <p className="mt-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {errorMsg}
          </p>
        )}

        {(canPay || canComplete || canNoShow || canCancel) && (
          <div className="mt-6 flex flex-wrap gap-3 border-t border-border pt-5">
            {canPay && !payOpen && (
              <Button onClick={() => setPayOpen(true)}>{t("payNow")}</Button>
            )}
            {canComplete && (
              <Button
                onClick={() =>
                  run("complete", () => completeBooking(booking.id))
                }
                disabled={busy !== null}
              >
                {busy === "complete" ? t("completing") : t("markComplete")}
              </Button>
            )}
            {canNoShow && (
              <>
                <Button
                  variant="outline"
                  onClick={() =>
                    run("no_show_student", () =>
                      reportNoShow(booking.id, "student"),
                    )
                  }
                  disabled={busy !== null}
                >
                  {busy === "no_show_student" ? t("reporting") : t("studentDidntShow")}
                </Button>
                <Button
                  variant="outline"
                  onClick={() =>
                    run("no_show_teacher", () =>
                      reportNoShow(booking.id, "teacher"),
                    )
                  }
                  disabled={busy !== null}
                >
                  {busy === "no_show_teacher" ? t("reporting") : t("couldntMakeIt")}
                </Button>
              </>
            )}
            {booking.canReschedule && !moveOpen && !cancelOpen && (
              <Button
                variant="outline"
                onClick={() => {
                  setErrorMsg(null);
                  setMoveOpen(true);
                }}
                disabled={busy !== null}
              >
                {t("moveLesson")}
              </Button>
            )}
            {canCancel && !cancelOpen && !moveOpen && (
              <Button
                variant="destructive"
                onClick={() => {
                  setErrorMsg(null);
                  setCancelOpen(true);
                }}
                disabled={busy !== null}
              >
                {t("cancelLesson")}
              </Button>
            )}
          </div>
        )}

        {booking.canReschedule && moveOpen && (
          <div
            role="group"
            aria-labelledby="move-heading"
            className="mt-4 border border-border bg-muted p-5"
          >
            <h2 id="move-heading" className="font-display text-lg">
              {t("moveHeading")}
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {t("moveNote", {
                price: formatMoney(booking.price, locale),
                left: 3 - booking.rescheduleCount,
                when: policy ? formatFull(policy.freeCancelUntil, viewerTz, locale) : "",
              })}
            </p>
            {!moveTo ? (
              <div className="mt-4">
                <SlotPicker
                  slug={booking.teacher.slug}
                  teacherTimezone={booking.teacher.timezone}
                  isTrial={booking.isTrial}
                  lessonTypeId={booking.lessonType?.id}
                  durations={[booking.durationMinutes as Duration]}
                  onPick={setMoveTo}
                />
              </div>
            ) : (
              <dl className="mt-4 divide-y divide-border border-y border-border text-sm">
                <Row label={t("currentTime")}>
                  {formatFull(booking.startAt, viewerTz, locale)}
                </Row>
                <Row label={t("newTime")}>{formatFull(moveTo.slot.startAt, viewerTz, locale)}</Row>
                {viewerTz !== booking.teacher.timezone && (
                  <Row label={t("teachersTime")}>
                    {formatFull(moveTo.slot.startAt, booking.teacher.timezone, locale)}
                  </Row>
                )}
              </dl>
            )}
            <div className="mt-4 flex flex-wrap gap-3">
              {moveTo && (
                <>
                  <Button onClick={() => void confirmMove()} disabled={busy !== null}>
                    {busy === "move" ? t("moving") : t("confirmMove")}
                  </Button>
                  <Button variant="outline" onClick={() => setMoveTo(null)} disabled={busy !== null}>
                    {t("changeTime")}
                  </Button>
                </>
              )}
              <Button
                variant="outline"
                onClick={() => {
                  setMoveOpen(false);
                  setMoveTo(null);
                }}
                disabled={busy !== null}
              >
                {t("keepTime")}
              </Button>
            </div>
          </div>
        )}

        {canCancel && cancelOpen && (
          <div
            role="group"
            aria-labelledby="cancel-heading"
            className="mt-4 border border-border bg-muted p-5"
          >
            <h2 id="cancel-heading" className="font-display text-lg">
              {t("cancelHeading")}
            </h2>
            <p
              className={
                forfeits
                  ? "mt-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
                  : "mt-2 text-sm text-muted-foreground"
              }
            >
              {booking.status !== "confirmed"
                ? t("cancelUnpaidNote")
                : !isStudent
                  ? t("cancelTeacherNote")
                  : forfeits
                    ? t("cancelForfeitNote", {
                        hours: FREE_CANCEL_HOURS,
                        price: formatMoney(booking.price, locale),
                        name: booking.teacher.displayName,
                      })
                    : t("cancelRefundNote", {
                        when: formatFull(policy!.freeCancelUntil, viewerTz, locale),
                      })}
            </p>
            <label htmlFor="cancel-reason" className="mt-4 block text-sm font-bold">
              {t("cancelReasonLabel")}
            </label>
            <Input
              id="cancel-reason"
              className="mt-1"
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              placeholder={t("cancelReasonPlaceholder")}
              maxLength={300}
            />
            <div className="mt-4 flex flex-wrap gap-3">
              <Button
                variant="destructive"
                onClick={() => void confirmCancel()}
                disabled={busy !== null}
              >
                {busy === "cancel"
                  ? t("cancelling")
                  : forfeits
                    ? t("cancelAndForfeit", { price: formatMoney(booking.price, locale) })
                    : booking.status === "confirmed"
                      ? t("cancelRefund")
                      : t("cancelLesson")}
              </Button>
              <Button
                variant="outline"
                onClick={() => setCancelOpen(false)}
                disabled={busy !== null}
              >
                {t("keepLesson")}
              </Button>
            </div>
          </div>
        )}
      </div>

      {canPay && payOpen && (
        <div className="mt-4">
          <PaymentForm
            bookingId={booking.id}
            amount={booking.price}
            onPaid={(b) => {
              setBooking(b);
              setPayOpen(false);
            }}
          />
        </div>
      )}

      <LessonResourcesPanel booking={booking} isTeacher={isTeacher} isStudent={isStudent} />

      {booking.review ? (
        <BookingReview
          rating={booking.review.rating}
          comment={booking.review.comment}
        />
      ) : (
        booking.canReview && (
          <ReviewPrompt
            bookingId={booking.id}
            teacherName={booking.teacher.displayName}
            onSubmitted={() => void refetch()}
          />
        )
      )}

      <DisputePanel booking={booking} onChanged={() => void refetch()} />
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
    <div className="flex justify-between gap-4 py-2.5">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-bold text-foreground">{children}</dd>
    </div>
  );
}
