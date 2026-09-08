/**
 * Auth data-access layer (browser). Thin wrappers over the generated client
 * that return view-models and throw a typed {@link AuthError} on failure so
 * forms can show the server's message.
 */

import { authedFetch, browserApi } from "./browser-client";
import { fromWireSession, fromWireUser } from "./mappers";
import type { AuthUser, Session } from "./types";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** A failed auth call. `code` is the backend's stable identifier when present. */
export class AuthError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "AuthError";
    this.code = code;
    this.status = status;
  }
}

type ErrorBody = { error?: { code?: string; message?: string } };

function toAuthError(error: unknown, status: number, fallback: string): AuthError {
  const body = error as ErrorBody | undefined;
  return new AuthError(
    body?.error?.message ?? fallback,
    body?.error?.code ?? "unknown",
    status,
  );
}

export async function register(input: {
  email: string;
  password: string;
  displayName: string;
}): Promise<Session> {
  const { data, error, response } = await browserApi.POST("/v1/auth/register", {
    body: {
      email: input.email,
      password: input.password,
      display_name: input.displayName,
    },
  });
  if (error || !data) {
    throw toAuthError(error, response.status, "Could not create your account.");
  }
  return fromWireSession(data);
}

export async function login(input: {
  email: string;
  password: string;
}): Promise<Session> {
  const { data, error, response } = await browserApi.POST("/v1/auth/login", {
    body: { email: input.email, password: input.password },
  });
  if (error || !data) {
    throw toAuthError(error, response.status, "Could not sign you in.");
  }
  return fromWireSession(data);
}

export async function logout(): Promise<void> {
  // 204 on success; even a failure here shouldn't block the client-side signout.
  await browserApi.POST("/v1/auth/logout", {});
}

export async function fetchCurrentUser(): Promise<AuthUser> {
  const { data, error, response } = await browserApi.GET("/v1/auth/me", {});
  if (error || !data) {
    throw toAuthError(error, response.status, "Could not load your account.");
  }
  return fromWireUser(data);
}

/**
 * Update the signed-in user's own account (currently just the display name).
 *
 * TODO: `PATCH /v1/auth/me` is being added to the backend; once it lands in
 * openapi.yaml, run `npm run gen:api` and switch this to `browserApi.PATCH`.
 */
export async function updateProfile(input: {
  displayName: string;
}): Promise<AuthUser> {
  const res = await authedFetch(`${baseUrl}/v1/auth/me`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ display_name: input.displayName }),
  });
  const body = await res.json().catch(() => undefined);
  if (!res.ok) {
    throw toAuthError(body, res.status, "Could not save your changes.");
  }
  return fromWireUser(body as Parameters<typeof fromWireUser>[0]);
}
