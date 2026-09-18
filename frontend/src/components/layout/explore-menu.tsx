"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ChevronDown } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

/** Languages we currently have teachers for. Mirrors the catalog. */
export const EXPLORE_LANGUAGES = ["en", "ru", "uz", "de", "ko", "tr"] as const;

/** Udemy's "Explore" — here, the languages you can learn plus the course catalog. */
export function ExploreMenu({ label }: { label: string }) {
  const t = useTranslations("nav");
  const tLang = useTranslations("languages");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className="hidden h-10 items-center gap-1 rounded-md px-2 text-sm text-foreground outline-none transition-colors hover:text-link focus-visible:ring-3 focus-visible:ring-ring/40 lg:inline-flex">
        {label}
        <ChevronDown className="size-4" aria-hidden />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-56">
        <DropdownMenuLabel className="text-xs text-muted-foreground">
          {t("exploreLanguages")}
        </DropdownMenuLabel>
        {EXPLORE_LANGUAGES.map((code) => (
          <DropdownMenuItem key={code} asChild>
            <Link href={`/teachers?lang=${code}`}>{tLang(code)}</Link>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link href="/teachers">{t("findTeacher")}</Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/courses/catalog">{t("findCourse")}</Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
