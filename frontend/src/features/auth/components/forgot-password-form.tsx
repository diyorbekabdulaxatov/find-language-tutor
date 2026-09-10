"use client";

import { useState } from "react";
import Link from "next/link";
import { requestPasswordReset } from "@/features/auth/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

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
          If <span className="font-medium">{email.trim()}</span> has an account,
          a reset link is on its way. It expires in an hour.
        </p>
        <p className="text-muted-foreground">
          Didn&apos;t get it? Check spam, or{" "}
          <button
            type="button"
            className="font-medium text-primary hover:underline"
            onClick={() => setSent(false)}
          >
            try another address
          </button>
          .
        </p>
        <Link href="/login" className="font-medium text-primary hover:underline">
          Back to sign in
        </Link>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      <p className="text-sm text-muted-foreground">
        Enter your email and we&apos;ll send you a link to choose a new password.
      </p>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="email">Email</Label>
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
        {busy ? "Sending…" : "Send reset link"}
      </Button>
      <p className="text-center text-sm text-muted-foreground">
        Remembered it?{" "}
        <Link href="/login" className="font-medium text-primary hover:underline">
          Sign in
        </Link>
      </p>
    </form>
  );
}
