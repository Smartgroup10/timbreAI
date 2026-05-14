import "./globals.css";
import type { Metadata } from "next";
import { Inter_Tight } from "next/font/google";
import { AuthProvider } from "../lib/auth-context";
import { LangProvider } from "../lib/i18n";
import { Chrome } from "../components/chrome";
import { ToastProvider } from "../components/toast";

const interTight = Inter_Tight({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
  variable: "--font-inter-tight",
});

export const metadata: Metadata = {
  title: "timbre.ai · Cada negocio merece su propio timbre",
  description: "Agentes de voz IA que llaman, agendan y responden con el tono de cada marca.",
  icons: { icon: "/brand/favicon.svg" },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es" className={interTight.variable}>
      <body>
        <LangProvider>
          <AuthProvider>
            <ToastProvider>
              <Chrome>{children}</Chrome>
            </ToastProvider>
          </AuthProvider>
        </LangProvider>
      </body>
    </html>
  );
}
