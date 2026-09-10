"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { formatMoney } from "@/lib/format";
import { AdminError, getUser, type AdminUserDetail } from "@/features/admin/api";
import { TeacherStatusBadge } from "./teacher-status-badge";
import { UserRolesCard } from "./user-roles-card";

function fmt(iso: string): string {
  return new Intl.DateTimeFormat("en-GB", {
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

  const reload = useCallback(() => {
    return getUser(id)
      .then((d) => {
        setData(d);
        setState("ready");
      })
      .catch((err) => {
        setErrMsg(
          err instanceof AdminError ? err.message : "Could not load that user.",
        );
        setState("error");
      });
  }, [id]);

  useEffect(() => {
    void reload();
  }, [reload]);

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

  const { user, roles, teacherProfile, bookings, paymentsSummary } = data;

  return (
    <div className="flex flex-col gap-6">
      <Back />

      <div className="rounded-2xl border border-border bg-card p-6">
        <h2 className="font-display text-2xl">{user.displayName}</h2>
        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <Row label="Email">{user.email}</Row>
          <Row label="User ID">
            <span className="font-mono text-xs">{user.id}</span>
          </Row>
          <Row label="Joined">{fmt(user.createdAt)}</Row>
        </dl>
      </div>

      <UserRolesCard userId={user.id} roles={roles} onChanged={reload} />

      <div className="rounded-2xl border border-border bg-card p-6">
        <h3 className="font-display text-lg">Teacher profile</h3>
        {teacherProfile ? (
          <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
            <Link
              href={`/admin/teachers/${teacherProfile.slug}`}
              className="font-medium text-primary hover:underline"
            >
              {teacherProfile.slug}
            </Link>
            <TeacherStatusBadge status={teacherProfile.status} />
            {teacherProfile.verified && (
              <span className="text-xs text-muted-foreground">verified</span>
            )}
          </div>
        ) : (
          <p className="mt-2 text-sm text-muted-foreground">None.</p>
        )}
      </div>

      <div className="rounded-2xl border border-border bg-card p-6">
        <h3 className="font-display text-lg">Payments</h3>
        <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-3">
          <Row label="Authorized">
            {formatMoney({
              amountMinor: paymentsSummary.authorizedMinor,
              currency: paymentsSummary.currency as "UZS" | "USD",
            })}
          </Row>
          <Row label="Captured">
            {formatMoney({
              amountMinor: paymentsSummary.capturedMinor,
              currency: paymentsSummary.currency as "UZS" | "USD",
            })}
          </Row>
          <Row label="Refunded">
            {formatMoney({
              amountMinor: paymentsSummary.refundedMinor,
              currency: paymentsSummary.currency as "UZS" | "USD",
            })}
          </Row>
        </dl>
      </div>

      <div className="rounded-2xl border border-border">
        <h3 className="px-6 pt-5 font-display text-lg">
          Bookings{" "}
          <span className="text-muted-foreground">({bookings.length})</span>
        </h3>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[32rem] text-sm">
            <thead className="bg-muted/50 text-left text-xs text-muted-foreground">
              <tr>
                <th className="px-6 py-2 font-medium">When</th>
                <th className="px-4 py-2 font-medium">As</th>
                <th className="px-4 py-2 font-medium">With</th>
                <th className="px-4 py-2 font-medium">Status</th>
                <th className="px-6 py-2 text-right font-medium">Price</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {bookings.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-6 py-6 text-center text-muted-foreground">
                    No bookings.
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
                      {fmt(b.startAt)}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{b.roleInBooking}</td>
                  <td className="px-4 py-3">{b.otherPartyName}</td>
                  <td className="px-4 py-3 text-muted-foreground">{b.status}</td>
                  <td className="px-6 py-3 text-right">{formatMoney(b.price)}</td>
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
  return (
    <Link
      href="/admin/users"
      className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
    >
      <ArrowLeft className="size-4" /> All users
    </Link>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className="font-medium">{children}</dd>
    </div>
  );
}
