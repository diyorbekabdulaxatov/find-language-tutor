"use client";

import { useState } from "react";
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
import { Button } from "@/components/ui/button";

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

  async function handlePay() {
    setPaying(true);
    setError(null);
    try {
      onPaid(await payBooking(bookingId, method));
    } catch (err) {
      setPaying(false);
      setError(
        err instanceof BookingError
          ? err.message
          : "Payment could not be processed. Please try again.",
      );
    }
  }

  return (
    <div className="flex flex-col gap-5 rounded-2xl border border-border bg-card p-6">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-xl">Payment</h2>
        <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
          <Lock className="size-3" /> Simulated — no real charge
        </span>
      </div>

      <div className="flex items-baseline justify-between rounded-xl bg-muted/60 px-4 py-3">
        <span className="text-sm text-muted-foreground">Amount due</span>
        <span className="font-display text-2xl">{formatMoney(amount)}</span>
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-medium">Payment method</legend>
        {TEST_METHODS.map((m) => (
          <label
            key={m.token}
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-xl border px-4 py-3 text-sm transition-colors",
              method === m.token
                ? "border-primary bg-accent/50"
                : "border-border hover:bg-muted/50",
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
            <span className="font-medium">{m.label}</span>
            <span className="ml-auto text-muted-foreground">{m.hint}</span>
          </label>
        ))}
      </fieldset>

      {error && (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          {error}
        </p>
      )}

      <Button size="lg" className="w-full" onClick={handlePay} disabled={paying}>
        {paying ? "Processing…" : `Pay ${formatMoney(amount)}`}
      </Button>
      <p className="text-center text-xs text-muted-foreground">
        The teacher is paid after the lesson takes place. Cancel any time before
        then for a full refund.
      </p>
    </div>
  );
}
