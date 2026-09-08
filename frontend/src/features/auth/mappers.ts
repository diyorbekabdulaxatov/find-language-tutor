/** wire (snake_case) -> view-model (camelCase) for the auth module. */

import type { components } from "@/lib/api/schema";
import type { AuthUser, Session } from "./types";

type WireUser = components["schemas"]["AuthUser"];
type WireAuthResponse = components["schemas"]["AuthResponse"];

export function fromWireUser(u: WireUser): AuthUser {
  return { id: u.id, email: u.email, displayName: u.display_name };
}

export function fromWireSession(r: WireAuthResponse): Session {
  return {
    user: fromWireUser(r.user),
    accessToken: r.access_token,
    expiresIn: r.expires_in,
  };
}
