"use client";

import { useMemo } from "react";
import { useAuth } from "@/features/auth/auth-context";
import { can as grants, hasAnyAdminAccess, type Permission } from "./permissions";

/**
 * `const { can, any } = useCan();`
 *  - `can(PERMISSIONS.teachersModerate)` — does the caller hold that permission?
 *  - `any` — true when the caller has any admin permission at all.
 *
 * The backend is the authority on every `/v1/admin` call; this only gates UI.
 */
export function useCan(): {
  can: (perm: Permission) => boolean;
  any: boolean;
} {
  const { user } = useAuth();
  const perms = user?.permissions;

  return useMemo(() => {
    const list = perms ?? [];
    return {
      can: (perm: Permission) => grants(list, perm),
      any: hasAnyAdminAccess(list),
    };
  }, [perms]);
}
