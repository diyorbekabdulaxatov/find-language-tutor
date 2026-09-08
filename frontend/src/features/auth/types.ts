/**
 * View-models for the auth module. The wire shapes (snake_case) come from the
 * generated client in `@/lib/api/schema`; these are the camelCase objects the
 * React tree actually consumes.
 */

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  role: "user" | "admin";
}

/** What a successful register / login / refresh gives the client. */
export interface Session {
  user: AuthUser;
  /** Short-lived HS256 access token. Held in memory only — never persisted. */
  accessToken: string;
  /** Access-token lifetime in seconds, straight from the server. */
  expiresIn: number;
}
