import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { GraduationCap } from "lucide-react";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { AccountMenu } from "@/features/auth/components/account-menu";

/**
 * Server component; the right-hand side (`<AccountMenu>`) is a client island
 * that reflects the session — log in / sign up when signed out, account
 * dropdown when signed in.
 */
export async function SiteHeader() {
  const t = await getTranslations("nav");
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
            {t("findTeacher")}
          </Link>
          <Link href="/courses/catalog" className="transition-colors hover:text-foreground">
            {t("findCourse")}
          </Link>
          <Link href="/#how-it-works" className="transition-colors hover:text-foreground">
            {t("howItWorks")}
          </Link>
          <Link href="/#teach" className="transition-colors hover:text-foreground">
            {t("teach")}
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-1.5">
          <LocaleSwitcher />
          <ThemeToggle />
          <AccountMenu />
        </div>
      </div>
    </header>
  );
}
