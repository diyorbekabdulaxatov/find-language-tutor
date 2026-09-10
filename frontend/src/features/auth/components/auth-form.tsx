"use client";

/**
 * Shared sign-in / sign-up form. On success it sends the visitor to the
 * `?next=` path (validated to be a local path) or falls back to /dashboard.
 * Server errors from {@link AuthError} are shown inline.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/features/auth/auth-context";
import { AuthError } from "@/features/auth/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Mode = "login" | "signup";

const COPY: Record<
  Mode,
  { cta: string; altText: string; altHref: string; altLink: string }
> = {
  login: {
    cta: "Log in",
    altText: "New to FindTutor?",
    altHref: "/signup",
    altLink: "Create an account",
  },
  signup: {
    cta: "Sign up",
    altText: "Already have an account?",
    altHref: "/login",
    altLink: "Log in",
  },
};

/** Only allow same-site, absolute-path redirects. */
function safeNext(next: string | null): string {
  if (next && next.startsWith("/") && !next.startsWith("//")) return next;
  return "/dashboard";
}

export function AuthForm({ mode }: { mode: Mode }) {
  const { login, register, status } = useAuth();
  const router = useRouter();
  const search = useSearchParams();
  const copy = COPY[mode];

  // Already signed in (e.g. hit /login from a bookmark) — move along.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace(safeNext(search.get("next")));
    }
  }, [status, router, search]);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      if (mode === "signup") {
        await register({ email, password, displayName });
      } else {
        await login(email, password);
      }
      router.replace(safeNext(search.get("next")));
    } catch (err) {
      setError(
        err instanceof AuthError
          ? err.message
          : "Something went wrong. Please try again.",
      );
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      {mode === "signup" && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="displayName">Name</Label>
          <Input
            id="displayName"
            name="name"
            autoComplete="name"
            required
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </div>
      )}

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

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          name="password"
          type="password"
          autoComplete={mode === "signup" ? "new-password" : "current-password"}
          required
          minLength={mode === "signup" ? 8 : undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {mode === "signup" ? (
          <p className="text-xs text-muted-foreground">At least 8 characters.</p>
        ) : (
          <Link
            href="/forgot-password"
            className="self-end text-xs font-medium text-primary hover:underline"
          >
            Forgot your password?
          </Link>
        )}
      </div>

      {error && (
        <p
          role="alert"
          className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          {error}
        </p>
      )}

      <Button type="submit" size="lg" disabled={submitting} className="mt-1 w-full">
        {submitting ? "One moment…" : copy.cta}
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        {copy.altText}{" "}
        <Link
          href={copy.altHref}
          className="font-medium text-primary hover:underline"
        >
          {copy.altLink}
        </Link>
      </p>
    </form>
  );
}
