import "./globals.css";
import type { Metadata } from "next";
import { AppShell } from "../components/app-shell";

export const metadata: Metadata = {
  title: "Atrium Calls",
  description: "AI calling platform for leasing and owner outreach"
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es">
      <body>
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}

