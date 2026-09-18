import type { Metadata } from "next";
import { NextIntlClientProvider } from "next-intl";
import { getLocale, getTranslations } from "next-intl/server";
import { Inter } from "next/font/google";
import "./globals.css";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthProvider } from "@/features/auth/auth-context";
import { EmailVerificationBanner } from "@/features/auth/components/email-verification-banner";
import { SiteHeader } from "@/components/layout/site-header";
import { SiteFooter } from "@/components/layout/site-footer";

/**
 * next/font self-hosts this — no runtime request to Google. It returns an
 * object with a `.variable` class that sets a CSS custom property; we hang it
 * on <html> and let globals.css map it to Tailwind's font tokens.
 *
 * One family for everything, like Udemy: Inter at 400 for body and 700 for
 * headings (`font-display` is an alias for the same face, bold).
 */
const sans = Inter({
  subsets: ["latin", "cyrillic"],
  variable: "--font-sans",
});

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("meta");
  return {
    title: { default: t("title"), template: "%s · FindTutor" },
    description: t("description"),
  };
}

export default async function RootLayout({ children }: LayoutProps<"/">) {
  const locale = await getLocale();
  return (
    // suppressHydrationWarning: next-themes sets the `class`/`style` on <html>
    // before React hydrates, so the server and client markup differ by design.
    <html
      lang={locale}
      suppressHydrationWarning
      className={`${sans.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <NextIntlClientProvider>
            <AuthProvider>
              <SiteHeader />
              <EmailVerificationBanner />
              <main className="flex-1">{children}</main>
              <SiteFooter />
            </AuthProvider>
          </NextIntlClientProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
