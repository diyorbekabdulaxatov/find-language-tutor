"use client";

import { useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { ShieldAlert } from "lucide-react";
import {
  BookingError,
  getBookingDisputes,
  raiseDispute,
  type Booking,
  type Dispute,
} from "@/features/bookings/api";
import { Button } from "@/components/ui/button";
import { intlLocale } from "@/lib/i18n";

const STATUS_LABEL = {
  open: "disputeOpen",
  resolved: "disputeResolved",
  rejected: "disputeRejected",
} as const;

function fmtDate(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(iso));
}

/**
 * The dispute section on a participant's booking detail. Shows the thread when
 * there's history, and a "Report a problem" form while `canRaiseDispute`.
 */
export function DisputePanel({
  booking,
  onChanged,
}: {
  booking: Booking;
  onChanged: () => void;
}) {
  const hasHistory = booking.openDispute !== null;
  const [thread, setThread] = useState<Dispute[]>([]);
  const [composing, setComposing] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const t = useTranslations("bookings");
  const locale = useLocale();

  useEffect(() => {
    if (!hasHistory) return;
    let alive = true;
    async function load() {
      try {
        const ds = await getBookingDisputes(booking.id);
        if (alive) setThread(ds);
      } catch {
        /* the thread is a nicety; the booking already carries the open one */
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [booking.id, hasHistory]);

  async function submit() {
    setBusy(true);
    setErr(null);
    try {
      await raiseDispute(booking.id, reason.trim());
      setComposing(false);
      setReason("");
      onChanged();
    } catch (e) {
      setErr(e instanceof BookingError ? e.message : t("couldNotDispute"));
    } finally {
      setBusy(false);
    }
  }

  // Nothing to show: no history and nothing the viewer can do.
  if (!hasHistory && !booking.canRaiseDispute) return null;

  const entries = thread.length
    ? thread
    : booking.openDispute
      ? [
          {
            id: booking.openDispute.id,
            status: booking.openDispute.status,
            reason: booking.openDispute.reason,
            resolution: "",
            raisedBy: { id: "", displayName: t("youOrOther") },
            resolvedBy: null,
            createdAt: booking.openDispute.createdAt,
            resolvedAt: null,
            bookingId: booking.id,
          } satisfies Dispute,
        ]
      : [];

  return (
    <div className="mt-4 border border-border bg-card p-6">
      <h2 className="flex items-center gap-2 font-display text-lg">
        <ShieldAlert className="size-4 text-link" /> {t("problemWithLesson")}
      </h2>

      {entries.length > 0 && (
        <ul className="mt-3 flex flex-col gap-3">
          {entries.map((d) => (
            <li
              key={d.id}
              className="border border-border bg-background p-3 text-sm"
            >
              <div className="flex items-center justify-between gap-2">
                <span
                  className={
                    d.status === "open"
                      ? "font-bold text-link"
                      : "font-bold text-muted-foreground"
                  }
                >
                  {t(STATUS_LABEL[d.status])}
                </span>
                <span className="text-xs text-muted-foreground">
                  {fmtDate(d.createdAt, locale)}
                </span>
              </div>
              <p className="mt-1 whitespace-pre-wrap">{d.reason}</p>
              {d.resolution && (
                <p className="mt-2 bg-muted px-2.5 py-1.5 text-xs">
                  <span className="font-bold">{t("moderator")}</span> {d.resolution}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}

      {booking.canRaiseDispute && !composing && (
        <Button
          variant="outline"
          className="mt-3"
          onClick={() => setComposing(true)}
        >
          {t("reportProblem")}
        </Button>
      )}

      {composing && (
        <div className="mt-3 flex flex-col gap-2">
          <label htmlFor="dispute-reason" className="text-sm font-bold">
            {t("whatWentWrong")}
          </label>
          <textarea
            id="dispute-reason"
            rows={4}
            maxLength={2000}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder={t("describeProblem")}
            className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <div className="flex gap-2">
            <Button disabled={busy || reason.trim() === ""} onClick={submit}>
              {busy ? t("submitting") : t("submitDispute")}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setComposing(false);
                setReason("");
                setErr(null);
              }}
            >
              {t("cancel")}
            </Button>
          </div>
        </div>
      )}

      {err && (
        <p className="mt-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {err}
        </p>
      )}
    </div>
  );
}
