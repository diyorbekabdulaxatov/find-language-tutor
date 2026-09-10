"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CheckCircle2, XCircle } from "lucide-react";
import { useAuth } from "@/features/auth/auth-context";
import { fetchCurrentUser, verifyEmail } from "@/features/auth/api";
import { Button } from "@/components/ui/button";

type State = "verifying" | "done" | "failed";

export function VerifyEmailPanel({ token }: { token: string }) {
  const { status, setUser } = useAuth();
  const [state, setState] = useState<State>("verifying");

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
        Confirming your email…
      </p>
    );
  }

  if (state === "failed") {
    return (
      <div className="flex flex-col items-center gap-4 text-center text-sm">
        <XCircle className="size-10 text-destructive" />
        <p>
          This link is invalid or has expired. Verification links last 24 hours
          and can only be used once.
        </p>
        <p className="text-muted-foreground">
          {status === "authenticated"
            ? "Open your dashboard to send yourself a fresh one."
            : "Sign in and we'll offer you a new link."}
        </p>
        <Button asChild variant="outline" className="w-full">
          <Link href={status === "authenticated" ? "/dashboard" : "/login"}>
            {status === "authenticated" ? "Go to dashboard" : "Sign in"}
          </Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-4 text-center text-sm">
      <CheckCircle2 className="size-10 text-primary" />
      <p>Your email is confirmed. Thanks!</p>
      <Button asChild className="w-full">
        <Link href={status === "authenticated" ? "/dashboard" : "/login"}>
          {status === "authenticated" ? "Go to dashboard" : "Sign in"}
        </Link>
      </Button>
    </div>
  );
}
