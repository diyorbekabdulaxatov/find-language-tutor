"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { CreditCard, Lock } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatMoney } from "@/lib/format";
import type { Money } from "@/types/teacher";
import {
  BookingError,
  payBooking,
  TEST_METHODS,
  type Booking,
  type MethodToken,
} from "@/features/bookings/api";

/**
 * The simulated payment step. There is no real PSP yet — the backend's fake
 * provider keys off the method token, so this is a picker over the test
 * methods rather than a card form. `onPaid` fires with the now-confirmed
 * booking.
 */
export function PaymentForm({
  bookingId,
  amount,
  onPaid,
}: {
  bookingId: string;
  amount: Money;
  onPaid: (booking: Booking) => void;
}) {
  const [method, setMethod] = useState<MethodToken>("pm_ok");
  const [error, setError] = useState<string | null>(null);
  const [paying, setPaying] = useState(false);
  const t = useTranslations("bookings");
  const locale = useLocale();
  const METHOD_LABEL = { pm_ok: "testCardOk", pm_decline: "testCardDeclined" } as const;

  async function handlePay() {
    setPaying(true);
    setError(null);
    try {
      onPaid(await payBooking(bookingId, method));
    } catch (err) {
      setPaying(false);
      setError(err instanceof BookingError ? err.message : t("paymentFailed"));
    }
  }

  return (
    <div className="flex flex-col gap-5 border border-border bg-card p-6">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-xl">{t("payment")}</h2>
        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
          <Lock className="size-3" /> {t("simulated")}
        </span>
      </div>

      <div className="flex items-baseline justify-between border-y border-border py-3">
        <span className="text-sm text-muted-foreground">{t("amountDue")}</span>
        <span className="font-display text-2xl">{formatMoney(amount, locale)}</span>
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-bold">{t("paymentMethod")}</legend>
        {TEST_METHODS.map((m) => (
          <label
            key={m.token}
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-md border px-4 py-3 text-sm transition-colors",
              method === m.token
                ? "border-foreground bg-accent"
                : "border-border hover:bg-muted",
            )}
          >
            <input
              type="radio"
              name="method"
              value={m.token}
              checked={method === m.token}
              onChange={() => {
                setMethod(m.token);
                setError(null);
              }}
              className="accent-primary"
            />
            <CreditCard className="size-4 text-muted-foreground" />
            <span className="font-bold">{t(METHOD_LABEL[m.token])}</span>
            <span className="ml-auto text-muted-foreground">{m.hint}</span>
          </label>
        ))}
      </fieldset>

      {error && (
        <p
          role="alert"
          className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          {error}
        </p>
      )}

      <button
        type="button"
        onClick={handlePay}
        disabled={paying}
        className="flex h-12 w-full items-center justify-center rounded-md bg-primary text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8] disabled:opacity-60"
      >
        {paying ? t("processing") : t("pay", { amount: formatMoney(amount, locale) })}
      </button>
      <p className="text-center text-xs text-muted-foreground">{t("teacherPaidAfter")}</p>
    </div>
  );
}
