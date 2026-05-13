"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { api, clearSession, getStoredUser, getToken, setSession, User } from "./api";

type AuthState = {
  user: User | null;
  ready: boolean;
  tenantOverride: string;
  setTenantOverride: (tenantId: string) => void;
  login: (email: string, password: string) => Promise<User>;
  logout: () => void;
  refresh: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);
  const [tenantOverride, setTenantOverrideState] = useState("");

  useEffect(() => {
    const stored = getStoredUser();
    if (stored && getToken()) {
      setUser(stored);
    }
    const savedOverride = typeof window !== "undefined" ? window.localStorage.getItem("callhub_tenant_override") : null;
    if (savedOverride) setTenantOverrideState(savedOverride);
    setReady(true);
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const res = await api.login(email, password);
    setSession(res.token, res.user);
    setUser(res.user);
    return res.user;
  }, []);

  const logout = useCallback(() => {
    clearSession();
    setUser(null);
    if (typeof window !== "undefined") {
      window.localStorage.removeItem("callhub_tenant_override");
      window.location.href = "/login";
    }
  }, []);

  const refresh = useCallback(async () => {
    try {
      const me = await api.me();
      setUser((prev) => ({ ...(prev ?? ({} as User)), ...me }));
    } catch {
      clearSession();
      setUser(null);
    }
  }, []);

  const setTenantOverride = useCallback((tenantId: string) => {
    setTenantOverrideState(tenantId);
    if (typeof window !== "undefined") {
      if (tenantId) window.localStorage.setItem("callhub_tenant_override", tenantId);
      else window.localStorage.removeItem("callhub_tenant_override");
    }
  }, []);

  const value = useMemo<AuthState>(
    () => ({ user, ready, tenantOverride, setTenantOverride, login, logout, refresh }),
    [user, ready, tenantOverride, setTenantOverride, login, logout, refresh],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

export function useTenantScope(): string | undefined {
  const { user, tenantOverride } = useAuth();
  if (user?.role === "platform_admin") {
    return tenantOverride || undefined;
  }
  return undefined;
}
