"use client";

import { useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { AuthError, resetPassword } from "@/features/auth/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function ResetPasswordForm({ token }: { token: string }) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const t = useTranslations("auth");
  const tCommon = useTranslations("common");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (password !== confirm) {
      setError(t("passwordsMismatch"));
      return;
    }
    setBusy(true);
    try {
      await resetPassword(token, password);
      setDone(true);
    } catch (err) {
      setError(err instanceof AuthError ? err.message : tCommon("somethingWrong"));
      setBusy(false);
    }
  }

  if (done) {
    return (
      <div className="flex flex-col gap-4 text-sm">
        <p>{t("resetDone")}</p>
        <Button asChild size="lg" className="w-full">
          <Link href="/login">{t("signIn")}</Link>
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="password">{t("newPassword")}</Label>
        <Input
          id="password"
          type="password"
          autoComplete="new-password"
          required
          minLength={8}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        <p className="text-xs text-muted-foreground">{t("atLeast8")}</p>
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="confirm">{t("confirmPassword")}</Label>
        <Input
          id="confirm"
          type="password"
          autoComplete="new-password"
          required
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
        />
      </div>

      {error && (
        <div
          role="alert"
          className="flex flex-col gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          <span>{error}</span>
          <Link href="/forgot-password" className="font-medium underline">
            {t("requestNewLink")}
          </Link>
        </div>
      )}

      <Button type="submit" size="lg" disabled={busy} className="mt-1 w-full">
        {busy ? t("saving") : t("changePassword")}
      </Button>
    </form>
  );
}
