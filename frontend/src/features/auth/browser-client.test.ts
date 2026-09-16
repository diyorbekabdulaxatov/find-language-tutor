import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authedFetch, refreshSession } from "./browser-client";
import { clearSession, getAccessToken, getAuthSnapshot, setSession } from "./auth-store";
import type { AuthUser } from "./types";

const user: AuthUser = {
  id: "u1",
  email: "a@example.com",
  displayName: "A",
  locale: "en",
  emailVerified: true,
  permissions: [],
};

const wireUser = {
  id: "u1",
  email: "a@example.com",
  display_name: "A",
  locale: "en",
  email_verified: true,
  permissions: [],
};

type Call = { url: string; auth: string | null };

/** A scripted fetch: each entry answers the next call to its path. */
function scriptFetch(script: Record<string, Array<() => Response>>) {
  const calls: Call[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const req = new Request(input as RequestInfo, init);
    const path = new URL(req.url).pathname;
    calls.push({ url: path, auth: req.headers.get("Authorization") });
    const next = script[path]?.shift();
    if (!next) throw new Error(`unexpected fetch ${path}`);
    return next();
  });
  vi.stubGlobal("fetch", fetchMock);
  return { calls, fetchMock };
}

const ok = (body: unknown) => () =>
  new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
const status = (code: number) => () => new Response(null, { status: code });
const refreshed = (token: string) => ok({ access_token: token, expires_in: 900, user: wireUser });

beforeEach(() => clearSession());
afterEach(() => vi.unstubAllGlobals());

describe("authedFetch", () => {
  it("attaches the in-memory access token as a Bearer header", async () => {
    setSession(user, "tok-1");
    const { calls } = scriptFetch({ "/v1/x": [status(200)] });
    await authedFetch("http://api.test/v1/x");
    expect(calls[0].auth).toBe("Bearer tok-1");
  });

  it("on a 401, refreshes once and retries with the new token", async () => {
    setSession(user, "stale");
    const { calls } = scriptFetch({
      "/v1/x": [status(401), status(200)],
      "/v1/auth/refresh": [refreshed("fresh")],
    });
    const res = await authedFetch("http://api.test/v1/x");
    expect(res.status).toBe(200);
    expect(calls.map((c) => c.url)).toEqual(["/v1/x", "/v1/auth/refresh", "/v1/x"]);
    expect(calls[2].auth).toBe("Bearer fresh");
    expect(getAccessToken()).toBe("fresh");
  });

  it("gives up after one failed refresh and clears the session", async () => {
    setSession(user, "stale");
    const { calls } = scriptFetch({
      "/v1/x": [status(401)],
      "/v1/auth/refresh": [status(401)],
    });
    const res = await authedFetch("http://api.test/v1/x");
    expect(res.status).toBe(401);
    expect(calls).toHaveLength(2); // no second retry, no refresh loop
    expect(getAuthSnapshot().user).toBeNull();
  });

  it("never tries to refresh a 401 from the refresh endpoint itself", async () => {
    const { calls } = scriptFetch({ "/v1/auth/refresh": [status(401)] });
    const res = await authedFetch("http://api.test/v1/auth/refresh", { method: "POST" });
    expect(res.status).toBe(401);
    expect(calls).toHaveLength(1);
  });

  it("shares one refresh across concurrent 401s", async () => {
    setSession(user, "stale");
    const { calls } = scriptFetch({
      "/v1/a": [status(401), status(200)],
      "/v1/b": [status(401), status(200)],
      "/v1/auth/refresh": [refreshed("fresh")],
    });
    const [a, b] = await Promise.all([
      authedFetch("http://api.test/v1/a"),
      authedFetch("http://api.test/v1/b"),
    ]);
    expect(a.status).toBe(200);
    expect(b.status).toBe(200);
    expect(calls.filter((c) => c.url === "/v1/auth/refresh")).toHaveLength(1);
  });

  it("retries with the original body intact", async () => {
    setSession(user, "stale");
    const bodies: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const req = new Request(input as RequestInfo, init);
        const path = new URL(req.url).pathname;
        if (path === "/v1/auth/refresh") return refreshed("fresh")();
        bodies.push(await req.text());
        return bodies.length === 1 ? status(401)() : status(200)();
      }),
    );
    await authedFetch("http://api.test/v1/x", { method: "POST", body: '{"k":1}' });
    expect(bodies).toEqual(['{"k":1}', '{"k":1}']);
  });
});

describe("refreshSession", () => {
  it("resolves false and clears the session when the cookie is gone", async () => {
    setSession(user, "tok");
    scriptFetch({ "/v1/auth/refresh": [status(401)] });
    expect(await refreshSession()).toBe(false);
    expect(getAccessToken()).toBeNull();
  });

  it("never throws on a network failure", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => { throw new TypeError("offline"); }));
    expect(await refreshSession()).toBe(false);
  });
});
