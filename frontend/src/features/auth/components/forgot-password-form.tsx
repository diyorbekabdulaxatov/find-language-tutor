"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { requestPasswordReset } from "@/features/auth/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const t = useTranslations("auth");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      await requestPasswordReset(email.trim());
    } catch {
      // The endpoint is 202 regardless; a network blip shouldn't leak either.
    }
    setSent(true);
    setBusy(false);
  }

  if (sent) {
    return (
      <div className="flex flex-col gap-4 text-sm">
        <p>
          {t.rich("forgotSent", {
            email: email.trim(),
            b: (chunks) => <span className="font-medium">{chunks}</span>,
          })}
        </p>
        <p className="text-muted-foreground">
          {t.rich("forgotNotReceived", {
            link: (chunks) => (
              <button
                type="button"
                className="font-medium text-primary hover:underline"
                onClick={() => setSent(false)}
              >
                {chunks}
              </button>
            ),
          })}
        </p>
        <Link href="/login" className="font-medium text-primary hover:underline">
          {t("backToSignIn")}
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <p className="text-sm text-muted-foreground">{t("forgotIntro")}</p>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="email">{t("email")}</Label>
        <Input
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </div>
      <Button type="submit" size="lg" disabled={busy} className="mt-1 w-full">
        {busy ? t("sending") : t("sendResetLink")}
      </Button>
      <p className="text-center text-sm text-muted-foreground">
        {t("rememberedIt")}{" "}
        <Link href="/login" className="font-medium text-primary hover:underline">
          {t("signIn")}
        </Link>
      </p>
    </form>
  );
}
