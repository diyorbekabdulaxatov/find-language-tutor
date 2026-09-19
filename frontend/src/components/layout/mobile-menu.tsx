"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Menu } from "lucide-react";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { useAuth } from "@/features/auth/auth-context";
import { EXPLORE_LANGUAGES } from "./explore-menu";

/** The hamburger drawer for phones — the header links plus the Explore list. */
export function MobileMenu() {
  const t = useTranslations("nav");
  const tLang = useTranslations("languages");
  const { status } = useAuth();
  const signedIn = status === "authenticated";

  const item =
    "block rounded-md px-3 py-2.5 text-base text-foreground transition-colors hover:bg-accent hover:text-accent-foreground";

  return (
    <Sheet>
      <SheetTrigger
        aria-label={t("menu")}
        className="-ml-1 grid size-10 place-items-center rounded-md text-foreground outline-none hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/40 lg:hidden"
      >
        <Menu className="size-6" />
      </SheetTrigger>
      <SheetContent side="left" className="w-80 overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="font-display text-xl text-primary">FindTutor</SheetTitle>
        </SheetHeader>
        <nav className="flex flex-col gap-1 px-2 pb-6">
          {!signedIn && (
            <>
              <SheetClose asChild>
                <Link href="/login" className={`${item} font-bold text-link`}>
                  {t("login")}
                </Link>
              </SheetClose>
              <SheetClose asChild>
                <Link href="/signup" className={`${item} font-bold text-link`}>
                  {t("signUp")}
                </Link>
              </SheetClose>
              <div className="my-2 border-t border-border" />
            </>
          )}
          {signedIn && (
            <>
              <SheetClose asChild>
                <Link href="/learn" className={item}>
                  {t("myLearning")}
                </Link>
              </SheetClose>
              <SheetClose asChild>
                <Link href="/bookings" className={item}>
                  {t("myBookings")}
                </Link>
              </SheetClose>
              <div className="my-2 border-t border-border" />
            </>
          )}
          <p className="px-3 pt-1 pb-1 text-xs font-bold text-muted-foreground">
            {t("exploreLanguages")}
          </p>
          {EXPLORE_LANGUAGES.map((code) => (
            <SheetClose asChild key={code}>
              <Link href={`/teachers?lang=${code}`} className={item}>
                {tLang(code)}
              </Link>
            </SheetClose>
          ))}
          <div className="my-2 border-t border-border" />
          <SheetClose asChild>
            <Link href="/teachers" className={item}>
              {t("findTeacher")}
            </Link>
          </SheetClose>
          <SheetClose asChild>
            <Link href="/courses/catalog" className={item}>
              {t("findCourse")}
            </Link>
          </SheetClose>
          <SheetClose asChild>
            <Link href="/#teach" className={item}>
              {t("teachOn")}
            </Link>
          </SheetClose>
        </nav>
      </SheetContent>
    </Sheet>
  );
}
