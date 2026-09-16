"use client";

/**
 * Shared sign-in / sign-up form. On success it sends the visitor to the
 * `?next=` path (validated to be a local path) or to a sensible default for
 * who they are — see postAuthDestination. Server errors from {@link AuthError}
 * are shown inline.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/features/auth/auth-context";
import { AuthError } from "@/features/auth/api";
import { postAuthDestination } from "@/features/auth/destination";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type Mode = "login" | "signup";

const COPY = {
  login: { cta: "logIn", altText: "newHere", altHref: "/signup", altLink: "createAccount" },
  signup: { cta: "signUp", altText: "haveAccount", altHref: "/login", altLink: "logIn" },
} as const;

export function AuthForm({ mode }: { mode: Mode }) {
  const { login, register, status } = useAuth();
  const router = useRouter();
  const search = useSearchParams();
  const copy = COPY[mode];
  const t = useTranslations("auth");
  const tCommon = useTranslations("common");
  const role = search.get("role");
  const destination = postAuthDestination({ mode, next: search.get("next"), role });
  // The other form (log in ↔ sign up) keeps the role and return path.
  const altHref = search.toString() ? `${copy.altHref}?${search}` : copy.altHref;

  // Already signed in (e.g. hit /login from a bookmark) — move along.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace(destination);
    }
  }, [status, router, destination]);

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
      router.replace(destination);
    } catch (err) {
      setError(err instanceof AuthError ? err.message : tCommon("somethingWrong"));
      setSubmitting(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4" noValidate>
      {mode === "signup" && role === "teacher" && (
        <p className="rounded-lg bg-primary/8 px-3 py-2 text-sm text-muted-foreground">{t("teacherSignupHint")}</p>
      )}
      {mode === "signup" && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="displayName">{t("name")}</Label>
          <Input
            id="displayName"
            name="name"
            autoComplete="name"
            maxLength={80}
            required
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
          />
        </div>
      )}

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

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="password">{t("password")}</Label>
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
          <p className="text-xs text-muted-foreground">{t("atLeast8")}</p>
        ) : (
          <Link
            href="/forgot-password"
            className="self-end text-xs font-medium text-primary hover:underline"
          >
            {t("forgotPassword")}
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
        {submitting ? t("oneMoment") : t(copy.cta)}
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        {t(copy.altText)}{" "}
        <Link
          href={altHref}
          className="font-medium text-primary hover:underline"
        >
          {t(copy.altLink)}
        </Link>
      </p>
    </form>
  );
}
