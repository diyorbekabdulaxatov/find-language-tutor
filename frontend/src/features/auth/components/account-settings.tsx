"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/features/auth/auth-context";
import { AuthError, updateProfile } from "@/features/auth/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function AccountSettings() {
  const { user, setUser } = useAuth();

  const [displayName, setDisplayName] = useState(user?.displayName ?? "");
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [error, setError] = useState<string | null>(null);
  const t = useTranslations("auth");

  if (!user) return null;

  const dirty = displayName.trim() !== user.displayName && displayName.trim() !== "";

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setStatus("saving");
    try {
      const updated = await updateProfile({ displayName: displayName.trim() });
      setUser(updated);
      setDisplayName(updated.displayName);
      setStatus("saved");
    } catch (err) {
      setStatus("idle");
      setError(err instanceof AuthError ? err.message : t("couldNotSave"));
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="font-display text-3xl sm:text-[2rem]">{t("settingsTitle")}</h1>
      <p className="mt-1 text-sm text-muted-foreground">{t("settingsIntro")}</p>

      <form
        onSubmit={handleSubmit}
        className="mt-8 flex flex-col gap-5 border border-border bg-card p-6"
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="displayName">{t("name")}</Label>
          <Input
            id="displayName"
            value={displayName}
            onChange={(e) => {
              setDisplayName(e.target.value);
              setStatus("idle");
            }}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="email">{t("email")}</Label>
          <Input id="email" value={user.email} disabled readOnly />
          <p className="text-xs text-muted-foreground">{t("emailChangeUnsupported")}</p>
        </div>

        {error && (
          <p
            role="alert"
            className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive"
          >
            {error}
          </p>
        )}

        <div className="flex items-center gap-3">
          <Button type="submit" disabled={!dirty || status === "saving"}>
            {status === "saving" ? t("saving") : t("saveChanges")}
          </Button>
          {status === "saved" && (
            <span className="text-sm text-muted-foreground">{t("saved")}</span>
          )}
        </div>
      </form>
    </div>
  );
}
