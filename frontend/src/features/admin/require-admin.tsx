"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useAuth } from "@/features/auth/auth-context";
import { useCan } from "./use-can";
import type { Permission } from "./permissions";

/**
 * Guard for the /admin area. Auth lives only in memory, so this is necessarily
 * client-side (the backend also enforces `RequirePermission` on every call):
 * unauthenticated visitors bounce to /login, signed-in users without any admin
 * permission get a plain 403.
 *
 * Pass `permission` to additionally require a specific one for a sub-page.
 */
export function RequireAdmin({
  children,
  permission,
}: {
  children: React.ReactNode;
  permission?: Permission;
}) {
  const { status } = useAuth();
  const { can, any } = useCan();
  const router = useRouter();
  const pathname = usePathname();
  const t = useTranslations("admin");

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace(`/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [status, router, pathname]);

  if (status === "loading" || status === "unauthenticated") {
    return (
      <div className="py-24 text-sm text-muted-foreground">
        {t("checkingSession")}
      </div>
    );
  }

  if (!any) {
    return <Denied body={t("adminsOnly")} />;
  }
  if (permission && !can(permission)) {
    return <Denied body={t("noPermission")} />;
  }

  return <>{children}</>;
}

function Denied({ body }: { body: string }) {
  const t = useTranslations("admin");
  return (
    <div className="mx-auto max-w-md py-24 text-center">
      <h1 className="font-display text-2xl">{t("notAuthorised")}</h1>
      <p className="mt-2 text-sm text-muted-foreground">{body}</p>
    </div>
  );
}
