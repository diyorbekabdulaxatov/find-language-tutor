import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { LocaleSwitcher } from "@/components/layout/locale-switcher";
import { HeaderSearch } from "@/components/layout/header-search";
import { ExploreMenu } from "@/components/layout/explore-menu";
import { MobileMenu } from "@/components/layout/mobile-menu";
import { AccountMenu } from "@/features/auth/components/account-menu";

/**
 * Udemy-shaped header: wordmark, "Explore" dropdown, one big pill search, a
 * couple of text links, then the session controls. Server component; the
 * search, the dropdown and the right-hand side are client islands.
 */
export async function SiteHeader() {
  const t = await getTranslations("nav");
  return (
    <header className="sticky top-0 z-40 bg-background shadow-card dark:border-b dark:border-border dark:shadow-none">
      <div className="mx-auto flex h-[72px] max-w-[1340px] items-center gap-3 px-4 sm:px-6">
        <MobileMenu />

        <Link href="/" className="flex shrink-0 items-center" aria-label="FindTutor">
          <span className="font-display text-[1.6rem] leading-none tracking-tight text-primary">
            FindTutor
          </span>
        </Link>

        <ExploreMenu label={t("explore")} />

        <HeaderSearch placeholder={t("searchPlaceholder")} />

        <nav className="hidden items-center gap-1 text-sm text-foreground xl:flex">
          <Link
            href="/courses/catalog"
            className="rounded-md px-3 py-2 transition-colors hover:text-link"
          >
            {t("findCourse")}
          </Link>
          <Link
            href="/#teach"
            className="rounded-md px-3 py-2 transition-colors hover:text-link"
          >
            {t("teachOn")}
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <AccountMenu />
          <LocaleSwitcher />
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}
