"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";

/**
 * Thin wrapper so the root layout (a server component) can stay a server
 * component — only this file needs "use client".
 *
 * `attribute="class"` makes next-themes toggle `class="dark"` on <html>, which
 * is what the `.dark { ... }` token block in globals.css keys off.
 */
export function ThemeProvider({
  children,
  ...props
}: React.ComponentProps<typeof NextThemesProvider>) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
