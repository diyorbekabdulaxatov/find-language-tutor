"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/features/auth/auth-context";

/**
 * Client guard for the /admin area. Auth lives only in memory, so this is
 * necessarily client-side (the backend also enforces RequireAdmin on every
 * call): unauthenticated visitors bounce to /login, signed-in non-admins are
 * shown a plain 403.
 */
export function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { status, user } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [status, router, pathname]);

  if (status === "loading" || status === "unauthenticated") {
    return (
      <div className="mx-auto max-w-6xl px-4 py-24 text-sm text-muted-foreground sm:px-6">
        Checking your session…
      </div>
    );
  }

  if (user?.role !== "admin") {
    return (
      <div className="mx-auto max-w-md px-4 py-24 text-center sm:px-6">
        <h1 className="font-display text-2xl">Not authorised</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          This area is for administrators only.
        </p>
      </div>
    );
  }

  return <>{children}</>;
}
