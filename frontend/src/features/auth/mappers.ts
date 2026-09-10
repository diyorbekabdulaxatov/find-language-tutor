/** wire (snake_case) -> view-model (camelCase) for the auth module. */

import type { components } from "@/lib/api/schema";
import type { AuthUser, Session } from "./types";

type WireUser = components["schemas"]["AuthUser"];
type WireAuthResponse = components["schemas"]["AuthResponse"];

export function fromWireUser(u: WireUser): AuthUser {
  // `permissions` lands with the RBAC schema regen; default to [] until then.
  const permissions = (u as { permissions?: string[] }).permissions ?? [];
  return {
    id: u.id,
    email: u.email,
    displayName: u.display_name,
    permissions,
  };
}

export function fromWireSession(r: WireAuthResponse): Session {
  return {
    user: fromWireUser(r.user),
    accessToken: r.access_token,
    expiresIn: r.expires_in,
  };
}
