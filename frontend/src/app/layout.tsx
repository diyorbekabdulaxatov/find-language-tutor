import type { Metadata } from "next";
import { Bricolage_Grotesque, Plus_Jakarta_Sans } from "next/font/google";
import "./globals.css";
import { ThemeProvider } from "@/components/theme-provider";
import { SiteHeader } from "@/components/layout/site-header";
import { SiteFooter } from "@/components/layout/site-footer";

/**
 * next/font self-hosts these — no runtime request to Google. Each call returns
 * an object with a `.variable` class that sets a CSS custom property; we hang
 * both on <html> and let globals.css map them to Tailwind's font tokens.
 *
 * Bricolage Grotesque: chunky, characterful display face for headings.
 * Plus Jakarta Sans: clean, friendly workhorse for UI and body. Both variable,
 * so `weight` is omitted and the font-weight utilities cover the whole range.
 */
const display = Bricolage_Grotesque({
  subsets: ["latin"],
  variable: "--font-display",
});

const sans = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-sans",
});

export const metadata: Metadata = {
  title: {
    default: "findtutor — book 1-on-1 language lessons",
    template: "%s · findtutor",
  },
  description:
    "Find a language teacher you click with and book paid 1-on-1 video lessons that fit your schedule.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    // suppressHydrationWarning: next-themes sets the `class`/`style` on <html>
    // before React hydrates, so the server and client markup differ by design.
    <html
      lang="en"
      suppressHydrationWarning
      className={`${display.variable} ${sans.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <SiteHeader />
          <main className="flex-1">{children}</main>
          <SiteFooter />
        </ThemeProvider>
      </body>
    </html>
  );
}
