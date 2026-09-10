"use client";

import { useState } from "react";
import { MailWarning, X } from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { resendVerification } from "@/features/auth/api";

/**
 * A slim strip shown to a signed-in user whose email isn't confirmed yet.
 * Nothing is blocked on verification — this is just a nudge. Dismissible for
 * the current page load.
 */
export function EmailVerificationBanner() {
  const { user, status } = useAuth();
  const [dismissed, setDismissed] = useState(false);
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

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
          <span>Verification email sent — check your inbox.</span>
        ) : (
          <span className="flex-1">
            Confirm your email ({user.email}) to secure your account.{" "}
            <button
              type="button"
              disabled={busy}
              onClick={resend}
              className="font-semibold underline underline-offset-2 disabled:opacity-60"
            >
              {busy ? "Sending…" : "Resend link"}
            </button>
          </span>
        )}
        <button
          type="button"
          aria-label="Dismiss"
          onClick={() => setDismissed(true)}
          className="ml-auto shrink-0 rounded p-0.5 hover:bg-star/15"
        >
          <X className="size-4" />
        </button>
      </div>
    </div>
  );
}
