import Link from "next/link";
import { GraduationCap } from "lucide-react";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { AccountMenu } from "@/features/auth/components/account-menu";

/**
 * Server component; the right-hand side (`<AccountMenu>`) is a client island
 * that reflects the session — log in / sign up when signed out, account
 * dropdown when signed in.
 */
export function SiteHeader() {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-6xl items-center gap-6 px-4 sm:px-6">
        <Link href="/" className="flex items-center gap-2">
          <span className="grid size-8 place-items-center rounded-lg bg-primary text-primary-foreground">
            <GraduationCap className="size-5" />
          </span>
          <span className="font-display text-xl tracking-tight text-foreground">
            FindTutor
          </span>
        </Link>

        <nav className="hidden items-center gap-6 text-sm font-medium text-muted-foreground sm:flex">
          <Link href="/teachers" className="transition-colors hover:text-foreground">
            Find a teacher
          </Link>
          <Link href="/#how-it-works" className="transition-colors hover:text-foreground">
            How it works
          </Link>
          <Link href="/#teach" className="transition-colors hover:text-foreground">
            Teach
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-1.5">
          <ThemeToggle />
          <AccountMenu />
        </div>
      </div>
    </header>
  );
}
