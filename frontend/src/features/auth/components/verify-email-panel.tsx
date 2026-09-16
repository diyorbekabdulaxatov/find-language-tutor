"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { CheckCircle2, XCircle } from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { fetchCurrentUser, verifyEmail } from "@/features/auth/api";
import { Button } from "@/components/ui/button";

type State = "verifying" | "done" | "failed";

export function VerifyEmailPanel({ token }: { token: string }) {
  const { status, setUser } = useAuth();
  const [state, setState] = useState<State>("verifying");
  const t = useTranslations("auth");

  useEffect(() => {
    let alive = true;
    async function run() {
      try {
        await verifyEmail(token);
        if (!alive) return;
        setState("done");
        // Refresh the cached user so the "verify your email" banner clears.
        if (status === "authenticated") {
          try {
            setUser(await fetchCurrentUser());
          } catch {
            /* non-fatal */
          }
        }
      } catch {
        if (alive) setState("failed");
      }
    }
    void run();
    return () => {
      alive = false;
    };
    // token is stable for the life of the page
  }, [token, status, setUser]);

  if (state === "verifying") {
    return (
      <p className="text-center text-sm text-muted-foreground">
        {t("verifying")}
      </p>
    );
  }

  if (state === "failed") {
    return (
      <div className="flex flex-col items-center gap-4 text-center text-sm">
        <XCircle className="size-10 text-destructive" />
        <p>{t("verifyFailed")}</p>
        <p className="text-muted-foreground">
          {status === "authenticated" ? t("verifyFailedAuthed") : t("verifyFailedAnon")}
        </p>
        <Button asChild variant="outline" className="w-full">
          <Link href={status === "authenticated" ? "/dashboard" : "/login"}>
            {status === "authenticated" ? t("goToDashboard") : t("signIn")}
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-4 text-center text-sm">
      <CheckCircle2 className="size-10 text-primary" />
      <p>{t("verifyDone")}</p>
      <Button asChild className="w-full">
        <Link href={status === "authenticated" ? "/dashboard" : "/login"}>
          {status === "authenticated" ? t("goToDashboard") : t("signIn")}
        </Link>
      </Button>
    </div>
  );
}
