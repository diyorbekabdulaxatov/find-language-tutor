"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { intlLocale } from "@/lib/i18n";
import { AdminError, getUser, type AdminUserDetail } from "@/features/admin/api";
import { TeacherStatusBadge } from "./teacher-status-badge";
import { UserRolesCard } from "./user-roles-card";

function fmt(iso: string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function UserDetail({ id }: { id: string }) {
  const [data, setData] = useState<AdminUserDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [errMsg, setErrMsg] = useState<string | null>(null);
  const t = useTranslations("admin");
  const tStatus = useTranslations("bookingStatus");
  const locale = useLocale();

  const reload = useCallback(() => {
    return getUser(id)
      .then((d) => {
        setData(d);
        setState("ready");
      })
      .catch((err) => {
        setErrMsg(err instanceof AdminError ? err.message : t("couldNotLoadUser"));
        setState("error");
      });
  }, [id, t]);

  useEffect(() => {
    void reload();
  }, [reload]);

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

  const { user, roles, teacherProfile, bookings, paymentsSummary } = data;

  return (
    <div className="flex flex-col gap-6">
      <Back />

      <div className="border border-border bg-card p-6">
        <h2 className="font-display text-2xl">{user.displayName}</h2>
        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <Row label={t("email")}>{user.email}</Row>
          <Row label={t("userId")}>
            <span className="font-mono text-xs">{user.id}</span>
          </Row>
          <Row label={t("joined")}>{fmt(user.createdAt, locale)}</Row>
        </dl>
      </div>

      <UserRolesCard userId={user.id} roles={roles} onChanged={reload} />

      <div className="border border-border bg-card p-6">
        <h3 className="font-display text-lg">{t("teacherProfile")}</h3>
        {teacherProfile ? (
          <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
            <Link
              href={`/admin/teachers/${teacherProfile.slug}`}
              className="font-bold text-link hover:underline"
            >
              {teacherProfile.slug}
            </Link>
            <TeacherStatusBadge status={teacherProfile.status} />
            {teacherProfile.verified && (
              <span className="text-xs text-muted-foreground">{t("verified")}</span>
            )}
          </div>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">{t("none")}</p>
        )}
      </div>

      <div className="border border-border bg-card p-6">
        <h3 className="font-display text-lg">{t("payments")}</h3>
        <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-3">
          <Row label={t("authorized")}>
            {formatMoney(
              { amountMinor: paymentsSummary.authorizedMinor, currency: paymentsSummary.currency as "UZS" | "USD" },
              locale,
            )}
          </Row>
          <Row label={t("captured")}>
            {formatMoney(
              { amountMinor: paymentsSummary.capturedMinor, currency: paymentsSummary.currency as "UZS" | "USD" },
              locale,
            )}
          </Row>
          <Row label={t("refunded")}>
            {formatMoney(
              { amountMinor: paymentsSummary.refundedMinor, currency: paymentsSummary.currency as "UZS" | "USD" },
              locale,
            )}
          </Row>
        </dl>
      </div>

      <div className="border border-border">
        <h3 className="px-6 pt-5 font-display text-lg">
          {t("bookings")}{" "}
          <span className="text-muted-foreground">({bookings.length})</span>
        </h3>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[32rem] text-sm">
            <thead className="bg-muted text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-6 py-2.5 font-bold">{t("colWhen")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colAs")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colWith")}</th>
                <th className="px-4 py-2.5 font-bold">{t("colStatus")}</th>
                <th className="px-6 py-2.5 text-right font-bold">{t("colPrice")}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {bookings.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-6 py-6 text-center text-muted-foreground">
                    {t("noBookings")}
                  </td>
                </tr>
              )}
              {bookings.map((b) => (
                <tr key={b.id}>
                  <td className="px-6 py-3">
                    <Link
                      href={`/bookings/${b.id}`}
                      className="hover:underline"
                    >
                      {fmt(b.startAt, locale)}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {b.roleInBooking === "student" ? t("student") : t("teacher")}
                  </td>
                  <td className="px-4 py-3">{b.otherPartyName}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {tStatus.has(b.status as never) ? tStatus(b.status as never) : b.status}
                  </td>
                  <td className="px-6 py-3 text-right">{formatMoney(b.price, locale)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function Back() {
  const t = useTranslations("admin");
  return (
    <Link
      href="/admin/users"
      className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
    >
      <ArrowLeft className="size-4" /> {t("allUsers")}
    </Link>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-bold">{children}</dd>
    </div>
  );
}
