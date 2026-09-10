"use client";

import { useCan } from "./use-can";
import type { Permission } from "./permissions";

/**
 * Renders `children` only if the caller holds `permission`; otherwise a short
 * "not authorised" note. The admin layout already gates on *any* admin access,
 * so this is the per-page check. The backend enforces the real thing.
 */
export function PermissionGate({
  permission,
  children,
}: {
  permission: Permission;
  children: React.ReactNode;
}) {
  const { can } = useCan();
  if (!can(permission)) {
    return (
      <p className="rounded-xl bg-muted px-4 py-8 text-center text-sm text-muted-foreground">
        You don&apos;t have permission to view this section.
      </p>
    );
  }
  return <>{children}</>;
}
