/**
 * View-models for the auth module. The wire shapes (snake_case) come from the
 * generated client in `@/lib/api/schema`; these are the camelCase objects the
 * React tree actually consumes.
 */

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  /** False until the account confirms its address via the emailed link. */
  emailVerified: boolean;
  /** RBAC permission keys the caller holds (union across their roles). Empty
   *  for an ordinary user; a non-empty list means they have some admin access. */
  permissions: string[];
}

/** What a successful register / login / refresh gives the client. */
export interface Session {
  user: AuthUser;
  /** Short-lived HS256 access token. Held in memory only — never persisted. */
  accessToken: string;
  /** Access-token lifetime in seconds, straight from the server. */
  expiresIn: number;
}
