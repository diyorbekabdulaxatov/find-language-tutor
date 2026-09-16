"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { MailWarning, X } from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { resendVerification } from "@/features/auth/api";

/**
 * A slim strip shown to a signed-in user whose email isn't confirmed yet.
 * Booking and buying still work unverified; submitting a teacher profile and
 * publishing a course do not (403 `email_not_verified`), and those screens
 * carry their own notice. Dismissible for the current page load.
 */
export function EmailVerificationBanner() {
  const { user, status } = useAuth();
  const [dismissed, setDismissed] = useState(false);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const t = useTranslations("auth");

  if (status !== "authenticated" || !user || user.emailVerified || dismissed) {
    return null;
  }

  async function resend() {
    setBusy(true);
    try {
      await resendVerification();
      setSent(true);
    } catch {
      setSent(true); // 202/409 both mean "nothing more to do here"
    }
    setBusy(false);
  }

  return (
    <div className="border-b border-star/30 bg-star/10 text-star">
      <div className="mx-auto flex max-w-6xl items-center gap-3 px-4 py-2 text-sm sm:px-6">
        <MailWarning className="size-4 shrink-0" />
        {sent ? (
          <span>{t("bannerSent")}</span>
        ) : (
          <span className="flex-1">
            {t("bannerConfirm", { email: user.email })}{" "}
            <button
              type="button"
              disabled={busy}
              onClick={resend}
              className="font-semibold underline underline-offset-2 disabled:opacity-60"
            >
              {busy ? t("sending") : t("resendLink")}
            </button>
          </span>
        )}
        <button
          type="button"
          aria-label={t("dismiss")}
          onClick={() => setDismissed(true)}
          className="ml-auto shrink-0 rounded p-0.5 hover:bg-star/15"
        >
          <X className="size-4" />
        </button>
      </div>
    </div>
  );
}
