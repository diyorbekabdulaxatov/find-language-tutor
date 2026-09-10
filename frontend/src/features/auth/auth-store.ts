/**
 * The client-side auth store: the signed-in user plus the access token, kept
 * in a module singleton in memory (never localStorage — an XSS'd page can read
 * localStorage, and the refresh token already lives in an HttpOnly cookie).
 *
 * It is a tiny external store so both React (via `useSyncExternalStore` in
 * AuthProvider) and non-React code (the fetch wrapper's refresh-retry) read and
 * write the same state.
 */

import type { AuthUser } from "./types";

export interface AuthSnapshot {
  user: AuthUser | null;
  accessToken: string | null;
}

const EMPTY: AuthSnapshot = { user: null, accessToken: null };

let snapshot: AuthSnapshot = EMPTY;
const listeners = new Set<() => void>();

export function getAuthSnapshot(): AuthSnapshot {
  return snapshot;
}

/** Server snapshot for `useSyncExternalStore` — always signed-out on the server. */
export function getServerAuthSnapshot(): AuthSnapshot {
  return EMPTY;
}

export function setSession(user: AuthUser, accessToken: string): void {
  snapshot = { user, accessToken };
  emit();
}

export function clearSession(): void {
  if (snapshot === EMPTY) return;
  snapshot = EMPTY;
  emit();
}

export function getAccessToken(): string | null {
  return snapshot.accessToken;
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function emit(): void {
  for (const listener of listeners) listener();
}
