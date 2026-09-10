"use client";

import { useState } from "react";
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
      setError(
        err instanceof AuthError
          ? err.message
          : "Could not save your changes. Please try again.",
      );
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-12 sm:px-6">
      <h1 className="font-display text-3xl">Account settings</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Manage the details on your FindTutor account.
      </p>

      <form
        onSubmit={handleSubmit}
        className="mt-8 flex flex-col gap-5 rounded-2xl border border-border bg-card p-6 shadow-soft"
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="displayName">Name</Label>
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
          <Label htmlFor="email">Email</Label>
          <Input id="email" value={user.email} disabled readOnly />
          <p className="text-xs text-muted-foreground">
            Email changes aren&apos;t supported yet.
          </p>
        </div>

        {error && (
          <p
            role="alert"
            className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
          >
            {error}
          </p>
        )}

        <div className="flex items-center gap-3">
          <Button type="submit" disabled={!dirty || status === "saving"}>
            {status === "saving" ? "Saving…" : "Save changes"}
          </Button>
          {status === "saved" && (
            <span className="text-sm text-muted-foreground">Saved.</span>
          )}
        </div>
      </form>
    </div>
  );
}
