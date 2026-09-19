"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
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

const DISPUTE_LABEL = {
  open: "disputeOpen",
  resolved: "disputeResolved",
  rejected: "disputeRejected",
} as const;

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
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
  const t = useTranslations("admin");
  const tPay = useTranslations("paymentStatus");
  const locale = useLocale();

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
        setErrMsg(err instanceof AdminError ? err.message : t("couldNotLoadBooking"));
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id, t]);

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
      setErrMsg(err instanceof AdminError ? err.message : t("couldNotForceCancel"));
    } finally {
      setBusy(false);
    }
  }

  if (state === "loading") {
    return <div className="h-96 animate-pulse bg-muted" />;
  }
  if (state === "error" || !data) {
    return (
      <div>
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
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

      <div className="border border-border bg-card p-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-display text-2xl">
            {data.teacher.displayName}{" "}
            <span className="text-muted-foreground">×</span>{" "}
            {data.student.displayName}
          </h2>
          <BookingStatusBadge status={data.status} />
        </div>

        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <Row label={t("when")}>{fmt(data.startAt, locale)}</Row>
          <Row label={t("length")}>
            {t(data.isTrial ? "minTrial" : "min", { count: data.durationMinutes })}
          </Row>
          <Row label={t("price")}>{formatMoney(data.price, locale)}</Row>
          <Row label={t("payment")}>
            {data.payment
              ? data.payment.status === "authorized"
                ? t("paymentAuthorizedAdmin")
                : data.payment.status === "failed"
                  ? t("paymentFailed")
                  : tPay(data.payment.status)
              : t("noPaymentIntent")}
          </Row>
          <Row label={t("teacher")}>
            <Link
              href={`/admin/teachers/${data.teacher.slug}`}
              className="text-primary hover:underline"
            >
              {data.teacher.slug}
            </Link>
          </Row>
          <Row label={t("student")}>
            <Link
              href={`/admin/users/${data.student.id}`}
              className="text-primary hover:underline"
            >
              {data.student.email}
            </Link>
          </Row>
          {data.meetingUrl && <Row label={t("meetingLink")}>{data.meetingUrl}</Row>}
          {data.noShowParty && (
            <Row label={t("noShow")}>
              {data.noShowParty === "student" ? t("student") : t("teacher")}
            </Row>
          )}
          {data.status === "cancelled" && (
            <>
              <Row label={t("cancelledLabel")}>
                {data.cancelledAt ? fmt(data.cancelledAt, locale) : "—"}
                {data.cancelledBy && ` ${t("cancelledBy", { who: data.cancelledBy })}`}
              </Row>
              <Row label={t("reason")}>{data.cancellationReason || "—"}</Row>
            </>
          )}
        </dl>
      </div>

      {/* Dispute thread */}
      <div className="border border-border bg-card p-6">
        <h3 className="font-display text-lg">
          {t("disputes")}{" "}
          <span className="text-muted-foreground">
            ({data.disputes.length})
          </span>
        </h3>
        {data.disputes.length === 0 ? (
          <p className="mt-2 text-sm text-muted-foreground">{t("noDisputes")}</p>
        ) : (
          <ul className="mt-3 flex flex-col gap-3">
            {data.disputes.map((d) => (
              <li
                key={d.id}
                className="rounded-xl border border-border bg-background/40 p-3 text-sm"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="font-bold">
                    {t("raisedBy", { status: t(DISPUTE_LABEL[d.status]), name: d.raisedBy.displayName })}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {fmt(d.createdAt, locale)}
                  </span>
                </div>
                <p className="mt-1 whitespace-pre-wrap">{d.reason}</p>
                {d.resolution && (
                  <p className="mt-2 rounded-lg bg-muted px-2.5 py-1.5 text-xs">
                    <span className="font-bold">
                      {d.resolvedBy?.displayName ?? t("moderator")}:
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
            {t.rich("resolveFromQueue", {
              link: (chunks) => (
                <Link href="/admin/disputes" className="text-primary hover:underline">
                  {chunks}
                </Link>
              ),
            })}
          </p>
        )}
      </div>

      {errMsg && (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {errMsg}
        </p>
      )}

      {canForceCancel && cancellable && (
        <div className="border border-border bg-card p-6">
          <h3 className="font-display text-lg">{t("forceCancel")}</h3>
          <p className="mt-1 text-sm text-muted-foreground">{t("forceCancelIntro")}</p>
          {confirming ? (
            <div className="mt-3 flex flex-col gap-3">
              <textarea
                rows={3}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder={t("forceCancelReason")}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={refund}
                  onChange={(e) => setRefund(e.target.checked)}
                  className="accent-primary"
                />
                {t("refundRelease")}
              </label>
              <div className="flex gap-2">
                <Button
                  variant="destructive"
                  disabled={busy || reason.trim() === ""}
                  onClick={doForceCancel}
                >
                  {busy ? t("cancelling") : t("confirmForceCancel")}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setConfirming(false);
                    setReason("");
                  }}
                >
                  {t("cancel")}
                </Button>
              </div>
            </div>
          ) : (
            <Button
              variant="destructive"
              className="mt-3"
              onClick={() => setConfirming(true)}
            >
              {t("forceCancelThis")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

function Back() {
  const t = useTranslations("admin");
  return (
    <Link
      href="/admin/bookings"
      className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
    >
      <ArrowLeft className="size-4" /> {t("allBookings")}
    </Link>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-bold break-words">{children}</dd>
    </div>
  );
}
