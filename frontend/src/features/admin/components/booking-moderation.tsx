"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import {
  AdminError,
  forceCancelBooking,
  getAdminBooking,
  type AdminBookingDetail,
} from "@/features/admin/api";
import { PERMISSIONS } from "@/features/admin/permissions";
import { useCan } from "@/features/admin/use-can";
import { BookingStatusBadge } from "@/features/bookings/components/booking-status-badge";
import { Button } from "@/components/ui/button";

const PAYMENT_LABEL: Record<string, string> = {
  requires_payment: "Not paid",
  authorized: "Held (authorized, not captured)",
  captured: "Released to teacher",
  refunded: "Refunded",
  failed: "Failed",
};

const DISPUTE_LABEL: Record<string, string> = {
  open: "Open",
  resolved: "Resolved",
  rejected: "Rejected",
};

function fmt(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function BookingModeration({ id }: { id: string }) {
  const { can } = useCan();
  const canForceCancel = can(PERMISSIONS.bookingsForceCancel);

  const [data, setData] = useState<AdminBookingDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errMsg, setErrMsg] = useState<string | null>(null);

  const [confirming, setConfirming] = useState(false);
  const [reason, setReason] = useState("");
  const [refund, setRefund] = useState(true);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const d = await getAdminBooking(id);
        if (!alive) return;
        setData(d);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setErrMsg(
          err instanceof AdminError
            ? err.message
            : "Could not load that booking.",
        );
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id]);

  async function reload() {
    setData(await getAdminBooking(id));
    setState("ready");
  }

  async function doForceCancel() {
    setBusy(true);
    setErrMsg(null);
    try {
      await forceCancelBooking(id, reason.trim(), refund);
      setConfirming(false);
      setReason("");
      await reload();
    } catch (err) {
      setErrMsg(
        err instanceof AdminError
          ? err.message
          : "Could not force-cancel the booking.",
      );
    } finally {
      setBusy(false);
    }
  }

  if (state === "loading") {
    return <div className="h-96 animate-pulse rounded-2xl bg-muted" />;
  }
  if (state === "error" || !data) {
    return (
      <div>
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
        <Back />
      </div>
    );
  }

  const cancellable =
    data.status === "pending_payment" || data.status === "confirmed";

  return (
    <div className="flex flex-col gap-6">
      <Back />

      <div className="rounded-2xl border border-border bg-card p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-display text-2xl">
            {data.teacher.displayName}{" "}
            <span className="text-muted-foreground">×</span>{" "}
            {data.student.displayName}
          </h2>
          <BookingStatusBadge status={data.status} />
        </div>

        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <Row label="When">{fmt(data.startAt)}</Row>
          <Row label="Length">
            {data.durationMinutes} min{data.isTrial && " · trial"}
          </Row>
          <Row label="Price">{formatMoney(data.price)}</Row>
          <Row label="Payment">
            {data.payment
              ? (PAYMENT_LABEL[data.payment.status] ?? data.payment.status)
              : "No payment intent"}
          </Row>
          <Row label="Teacher">
            <Link
              href={`/admin/teachers/${data.teacher.slug}`}
              className="text-primary hover:underline"
            >
              {data.teacher.slug}
            </Link>
          </Row>
          <Row label="Student">
            <Link
              href={`/admin/users/${data.student.id}`}
              className="text-primary hover:underline"
            >
              {data.student.email}
            </Link>
          </Row>
          {data.meetingUrl && <Row label="Meeting link">{data.meetingUrl}</Row>}
          {data.noShowParty && (
            <Row label="No-show">
              {data.noShowParty === "student" ? "Student" : "Teacher"}
            </Row>
          )}
          {data.status === "cancelled" && (
            <>
              <Row label="Cancelled">
                {data.cancelledAt ? fmt(data.cancelledAt) : "—"}
                {data.cancelledBy && ` · by ${data.cancelledBy}`}
              </Row>
              <Row label="Reason">{data.cancellationReason || "—"}</Row>
            </>
          )}
        </dl>
      </div>

      {/* Dispute thread */}
      <div className="rounded-2xl border border-border bg-card p-6">
        <h3 className="font-display text-lg">
          Disputes{" "}
          <span className="text-muted-foreground">
            ({data.disputes.length})
          </span>
        </h3>
        {data.disputes.length === 0 ? (
          <p className="mt-2 text-sm text-muted-foreground">
            No disputes on this booking.
          </p>
        ) : (
          <ul className="mt-3 flex flex-col gap-3">
            {data.disputes.map((d) => (
              <li
                key={d.id}
                className="rounded-xl border border-border bg-background/40 p-3 text-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-medium">
                    {DISPUTE_LABEL[d.status] ?? d.status} · raised by{" "}
                    {d.raisedBy.displayName}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {fmt(d.createdAt)}
                  </span>
                </div>
                <p className="mt-1 whitespace-pre-wrap">{d.reason}</p>
                {d.resolution && (
                  <p className="mt-2 rounded-lg bg-muted px-2.5 py-1.5 text-xs">
                    <span className="font-medium">
                      {d.resolvedBy?.displayName ?? "Moderator"}:
                    </span>{" "}
                    {d.resolution}
                  </p>
                )}
              </li>
            ))}
          </ul>
        )}
        {data.disputes.some((d) => d.status === "open") && (
          <p className="mt-3 text-xs text-muted-foreground">
            Resolve open disputes from the{" "}
            <Link href="/admin/disputes" className="text-primary hover:underline">
              Disputes queue
            </Link>
            .
          </p>
        )}
      </div>

      {errMsg && (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
      )}

      {canForceCancel && cancellable && (
        <div className="rounded-2xl border border-border bg-card p-6">
          <h3 className="font-display text-lg">Force-cancel</h3>
          <p className="mt-1 text-sm text-muted-foreground">
            Cancels the booking even though neither participant asked. Both
            parties are emailed.
          </p>
          {confirming ? (
            <div className="mt-3 flex flex-col gap-3">
              <textarea
                rows={3}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Reason (stored on the booking, shown to both parties)"
                className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={refund}
                  onChange={(e) => setRefund(e.target.checked)}
                  className="accent-primary"
                />
                Refund / release the payment
              </label>
              <div className="flex gap-2">
                <Button
                  variant="destructive"
                  disabled={busy || reason.trim() === ""}
                  onClick={doForceCancel}
                >
                  {busy ? "Cancelling…" : "Confirm force-cancel"}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setConfirming(false);
                    setReason("");
                  }}
                >
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <Button
              variant="destructive"
              className="mt-3"
              onClick={() => setConfirming(true)}
            >
              Force-cancel this booking
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

function Back() {
  return (
    <Link
      href="/admin/bookings"
      className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="size-4" /> All bookings
    </Link>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-medium break-words">{children}</dd>
    </div>
  );
}
