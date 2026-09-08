"use client";

/**
 * Client-side route guard. Auth state lives only in memory, so protection is
 * necessarily client-side: while the silent refresh runs we show a fallback,
 * and an unauthenticated visitor is bounced to /login with a `next` param so
 * they land back here after signing in.
 */

import { useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "./auth-context";

export function RequireUser({ children }: { children: React.ReactNode }) {
  const { status } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();

  useEffect(() => {
    if (status !== "unauthenticated") return;
    const here = search.toString() ? `${pathname}?${search}` : pathname;
    router.replace(`/login?next=${encodeURIComponent(here)}`);
  }, [status, router, pathname, search]);

  if (status !== "authenticated") {
    return (
      <div className="mx-auto max-w-6xl px-4 py-24 text-sm text-muted-foreground sm:px-6">
        Checking your session…
      </div>
    );
  }

  return <>{children}</>;
}
