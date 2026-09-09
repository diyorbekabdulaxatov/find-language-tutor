"use client";

import { useEffect, useState } from "react";
import { ShieldAlert } from "lucide-react";
import {
  BookingError,
  getBookingDisputes,
  raiseDispute,
  type Booking,
  type Dispute,
} from "@/features/bookings/api";
import { Button } from "@/components/ui/button";

const STATUS_LABEL: Record<Dispute["status"], string> = {
  open: "Open — a moderator is reviewing this",
  resolved: "Resolved in favour of the person who raised it",
  rejected: "Reviewed — closed without action",
};

function fmtDate(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
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
      setErr(
        e instanceof BookingError
          ? e.message
          : "Could not open the dispute. Try again.",
      );
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
            raisedBy: { id: "", displayName: "You or the other participant" },
            resolvedBy: null,
            createdAt: booking.openDispute.createdAt,
            resolvedAt: null,
            bookingId: booking.id,
          } satisfies Dispute,
        ]
      : [];

  return (
    <div className="mt-4 rounded-2xl border border-border bg-card p-6">
      <h2 className="inline-flex items-center gap-2 font-display text-lg">
        <ShieldAlert className="size-4 text-coral" /> Problem with this lesson
      </h2>

      {entries.length > 0 && (
        <ul className="mt-3 flex flex-col gap-3">
          {entries.map((d) => (
            <li
              key={d.id}
              className="rounded-xl border border-border bg-background/40 p-3 text-sm"
            >
              <div className="flex items-center justify-between gap-2">
                <span
                  className={
                    d.status === "open"
                      ? "font-medium text-coral"
                      : "font-medium text-muted-foreground"
                  }
                >
                  {STATUS_LABEL[d.status]}
                </span>
                <span className="text-xs text-muted-foreground">
                  {fmtDate(d.createdAt)}
                </span>
              </div>
              <p className="mt-1 whitespace-pre-wrap">{d.reason}</p>
              {d.resolution && (
                <p className="mt-2 rounded-lg bg-muted px-2.5 py-1.5 text-xs">
                  <span className="font-medium">Moderator:</span> {d.resolution}
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
          Report a problem
        </Button>
      )}

      {composing && (
        <div className="mt-3 flex flex-col gap-2">
          <label htmlFor="dispute-reason" className="text-sm font-medium">
            What went wrong?
          </label>
          <textarea
            id="dispute-reason"
            rows={4}
            maxLength={2000}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Describe what happened. A moderator will review it."
            className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          />
          <div className="flex gap-2">
            <Button disabled={busy || reason.trim() === ""} onClick={submit}>
              {busy ? "Submitting…" : "Submit dispute"}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setComposing(false);
                setReason("");
                setErr(null);
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}

      {err && (
        <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {err}
        </p>
      )}
    </div>
  );
}
