"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { useAuth } from "../lib/auth-context";
import { AppShell } from "./app-shell";

const PUBLIC_PATHS = new Set(["/login"]);

export function Chrome({ children }: { children: React.ReactNode }) {
  const { user, ready } = useAuth();
  const pathname = usePathname() || "/";
  const router = useRouter();
  const isPublic = PUBLIC_PATHS.has(pathname);

  useEffect(() => {
    if (!ready) return;
    if (!user && !isPublic) {
      router.replace("/login");
    } else if (user && isPublic) {
      router.replace("/portal");
    }
  }, [ready, user, isPublic, router]);

  if (!ready) {
    return <div className="boot-screen">Cargando…</div>;
  }

  if (isPublic) {
    return <div className="public-shell">{children}</div>;
  }

  if (!user) {
    return <div className="boot-screen">Redirigiendo…</div>;
  }

  return <AppShell>{children}</AppShell>;
}
