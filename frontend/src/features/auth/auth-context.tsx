"use client";

/**
 * AuthProvider owns the client session: it bootstraps once on mount by trying a
 * silent refresh (the `ftr_session` cookie survives reloads even though the
 * in-memory access token does not), then exposes the current user plus the
 * auth actions. Components read it with {@link useAuth}.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  clearSession,
  getAuthSnapshot,
  getServerAuthSnapshot,
  setSession,
  subscribe,
} from "./auth-store";
import { refreshSession } from "./browser-client";
import * as authApi from "./api";
import type { AuthUser } from "./types";

type Status = "loading" | "authenticated" | "unauthenticated";

interface AuthContextValue {
  user: AuthUser | null;
  status: Status;
  login: (email: string, password: string) => Promise<void>;
  register: (input: {
    email: string;
    password: string;
    displayName: string;
  }) => Promise<void>;
  logout: () => Promise<void>;
  /** Patch the cached user after a profile edit — no network call. */
  setUser: (user: AuthUser) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const snapshot = useSyncExternalStore(
    subscribe,
    getAuthSnapshot,
    getServerAuthSnapshot,
  );

  const [bootstrapped, setBootstrapped] = useState(false);
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    refreshSession().finally(() => setBootstrapped(true));
  }, []);

  const status: Status = !bootstrapped
    ? "loading"
    : snapshot.user
      ? "authenticated"
      : "unauthenticated";

  const login = useCallback(async (email: string, password: string) => {
    const session = await authApi.login({ email, password });
    setSession(session.user, session.accessToken);
  }, []);

  const register = useCallback(
    async (input: { email: string; password: string; displayName: string }) => {
      const session = await authApi.register(input);
      setSession(session.user, session.accessToken);
    },
    [],
  );

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } finally {
      clearSession();
    }
  }, []);

  const setUser = useCallback(
    (user: AuthUser) => {
      if (snapshot.accessToken) setSession(user, snapshot.accessToken);
    },
    [snapshot.accessToken],
  );

  const value = useMemo<AuthContextValue>(
    () => ({ user: snapshot.user, status, login, register, logout, setUser }),
    [snapshot.user, status, login, register, logout, setUser],
  );

  return <AuthContext value={value}>{children}</AuthContext>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
