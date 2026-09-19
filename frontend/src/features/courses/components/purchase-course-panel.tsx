"use client";

import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { CreditCard, Lock } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/format";
import { CourseError, purchaseCourse } from "@/features/courses/api";
import type { CourseEnrollment } from "@/features/courses/types";
import type { Money } from "@/types/teacher";

/** Same deterministic fake payment provider as the booking flow, but courses
 *  capture immediately on purchase — `pm_capture_fail` is a meaningfully
 *  different, retryable outcome here (vs. bookings, where capture only
 *  happens later at lesson completion). */
const TEST_METHODS = [
  { token: "pm_ok", label: "testCardOk", hint: "•••• 4242" },
  { token: "pm_decline", label: "testCardDeclined", hint: "•••• 0002" },
  { token: "pm_capture_fail", label: "testCardCaptureFail", hint: "•••• 0341" },
] as const;

type T = ReturnType<typeof useTranslations<"courses">>;

function purchaseErrorMessage(err: unknown, t: T): string {
  if (!(err instanceof CourseError)) return t("purchaseFailed");
  switch (err.code) {
    case "payment_failed":
      return t("paymentDeclined");
    case "capture_failed":
      return t("captureFailed");
    case "purchase_unavailable":
      return t("purchaseUnavailable");
    default:
      return err.message;
  }
}

/**
 * The buy / enroll panel on a course's public landing page. A free course is
 * one click; a paid course collects a (fake, demo-only) test payment method
 * first — mirrors `PaymentForm`'s picker-over-test-methods UX from the
 * booking flow, since there's no real PSP yet.
 */
export function PurchaseCoursePanel({
  courseId,
  price,
  onPurchased,
}: {
  courseId: string;
  price: Money;
  onPurchased: (enrollment: CourseEnrollment) => void;
}) {
  const free = price.amountMinor === 0;
  const [method, setMethod] = useState<(typeof TEST_METHODS)[number]["token"]>("pm_ok");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const t = useTranslations("courses");
  const locale = useLocale();

  async function handlePurchase() {
    setBusy(true);
    setError(null);
    try {
      const { enrollment } = await purchaseCourse(courseId, free ? undefined : method);
      onPurchased(enrollment);
    } catch (err) {
      setError(purchaseErrorMessage(err, t));
    } finally {
      setBusy(false);
    }
  }

  if (free) {
    return (
      <div className="flex flex-col gap-3">
        {error && (
          <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
        <Button size="lg" className="w-full" onClick={() => void handlePurchase()} disabled={busy}>
          {busy ? t("enrolling") : t("enrollFree")}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Lock className="size-3" /> {t("simulated")}
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-1 text-sm font-bold">{t("paymentMethod")}</legend>
        {TEST_METHODS.map((m) => (
          <label
            key={m.token}
            className={cn(
              "flex cursor-pointer items-center gap-3 rounded-md border px-3.5 py-2.5 text-sm transition-colors",
              method === m.token ? "border-primary bg-accent/50" : "border-border hover:bg-muted/50",
            )}
          >
            <input
              type="radio"
              name="course-method"
              value={m.token}
              checked={method === m.token}
              onChange={() => {
                setMethod(m.token);
                setError(null);
              }}
              className="accent-primary"
            />
            <CreditCard className="size-4 text-muted-foreground" />
            <span className="font-bold">{t(m.label)}</span>
            <span className="ml-auto text-muted-foreground">{m.hint}</span>
          </label>
        ))}
      </fieldset>

      {error && (
        <p role="alert" className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      <Button size="lg" className="w-full" onClick={() => void handlePurchase()} disabled={busy}>
        {busy ? t("processing") : t("buyFor", { price: formatMoney(price, locale) })}
      </Button>
    </div>
  );
}
