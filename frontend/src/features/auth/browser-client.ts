/**
 * Browser-side HTTP for the Go backend.
 *
 * Unlike `@/lib/api/client` (server components, no credentials), everything
 * here runs in the browser: the in-memory access token goes out as a Bearer
 * header, the `ftr_session` cookie rides along (`credentials: "include"`), and
 * a 401 triggers exactly one transparent refresh-token rotation + retry.
 * Concurrent 401s share a single refresh call.
 *
 * `browserApi` is the typed client for documented endpoints; `authedFetch` is
 * the same transport for the occasional hand-rolled call.
 */

import createClient from "openapi-fetch";
import type { paths } from "@/lib/api/schema";
import { clearSession, getAccessToken, setSession } from "./auth-store";
import { fromWireUser } from "./mappers";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

const REFRESH_PATH = "/v1/auth/refresh";

/** In-flight refresh, shared so a burst of 401s triggers only one rotation. */
let refreshInFlight: Promise<boolean> | null = null;

/**
 * Rotate the refresh cookie. On success the new access token + user land in the
 * auth store and this resolves true; on failure the session is cleared and it
 * resolves false. Never throws.
 */
export function refreshSession(): Promise<boolean> {
  refreshInFlight ??= doRefresh().finally(() => {
    refreshInFlight = null;
  });
  return refreshInFlight;
}

type RefreshBody =
  paths["/v1/auth/refresh"]["post"]["responses"]["200"]["content"]["application/json"];

async function doRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${baseUrl}${REFRESH_PATH}`, {
      method: "POST",
      credentials: "include",
    });
    if (!res.ok) {
      clearSession();
      return false;
    }
    const body = (await res.json()) as RefreshBody;
    setSession(fromWireUser(body.user), body.access_token);
    return true;
  } catch {
    clearSession();
    return false;
  }
}

/**
 * `fetch` with the Bearer header attached and a single refresh-and-retry on
 * 401. Accepts the same arguments as `fetch`; openapi-fetch calls it with a
 * fully-formed `Request`, and hand-rolled callers can pass `(url, init)`.
 */
export const authedFetch: typeof fetch = async (input, init) => {
  const request = new Request(input as RequestInfo, init);
  applyAuth(request);

  const retryable = request.clone();
  const res = await fetch(request);

  if (res.status !== 401) return res;
  if (new URL(retryable.url).pathname.endsWith(REFRESH_PATH)) return res;

  const refreshed = await refreshSession();
  if (!refreshed) return res;

  applyAuth(retryable);
  return fetch(retryable);
};

function applyAuth(request: Request): void {
  const token = getAccessToken();
  if (token) request.headers.set("Authorization", `Bearer ${token}`);
}

export const browserApi = createClient<paths>({
  baseUrl,
  credentials: "include",
  fetch: authedFetch,
});
