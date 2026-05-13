import "./globals.css";
import type { Metadata } from "next";
import { GeistSans } from "geist/font/sans";
import { GeistMono } from "geist/font/mono";
import { AuthProvider } from "../lib/auth-context";
import { Chrome } from "../components/chrome";
import { ToastProvider } from "../components/toast";

export const metadata: Metadata = {
  title: "CallHub",
  description: "AI calling platform for leasing and owner outreach",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es" className={`${GeistSans.variable} ${GeistMono.variable}`}>
      <body>
        <AuthProvider>
          <ToastProvider>
            <Chrome>{children}</Chrome>
          </ToastProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
