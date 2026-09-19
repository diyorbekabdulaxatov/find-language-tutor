"use client";

import Link from "next/link";
import { useState } from "react";
import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { TeacherSummary } from "@/types/teacher";
import { languageName } from "@/lib/i18n";
import { TeacherCard } from "./teacher-card";

/**
 * Udemy's "All the skills you need in one place" block: a row of underlined
 * tabs (one per language), a short blurb, an "Explore" outline button, and a
 * row of cards for the selected tab. Everything is already in memory — the
 * home page ships the recommended teachers once and this only filters them.
 */
export function LanguageTabs({
  teachers,
  languages,
}: {
  teachers: TeacherSummary[];
  languages: { code: string; name: string; count: number }[];
}) {
  const t = useTranslations("home");
  const tLang = useTranslations("languages");
  const [active, setActive] = useState(languages[0]?.code ?? "en");
  const current = languages.find((l) => l.code === active) ?? languages[0];
  const shown = teachers.filter((tc) => tc.teaches.some((l) => l.code === active)).slice(0, 5);

  if (!current) return null;

  return (
    <div>
      <div
        role="tablist"
        className="-mx-4 flex gap-6 overflow-x-auto border-b border-border px-4 sm:mx-0 sm:px-0"
      >
        {languages.map((l) => (
          <button
            key={l.code}
            role="tab"
            aria-selected={l.code === active}
            onClick={() => setActive(l.code)}
            className={cn(
              "-mb-px shrink-0 border-b-2 py-3 text-base font-bold whitespace-nowrap transition-colors",
              l.code === active
                ? "border-foreground text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground",
            )}
          >
            {languageName(tLang, l)}
          </button>
        ))}
      </div>

      <div className="mt-6 border border-border bg-muted p-6 sm:p-8">
        <h3 className="font-display text-xl">
          {t("tabTitle", { language: languageName(tLang, current) })}
        </h3>
        <p className="mt-2 max-w-2xl text-sm text-muted-foreground">
          {t("tabBody", { count: current.count, language: languageName(tLang, current) })}
        </p>
        <Link
          href={`/teachers?lang=${current.code}`}
          className="mt-4 inline-flex h-10 items-center rounded-md border border-foreground bg-background px-3 text-sm font-bold text-foreground transition-colors hover:bg-accent"
        >
          {t("exploreLanguage", { language: languageName(tLang, current) })}
        </Link>

        <div className="-mx-6 mt-6 flex gap-4 overflow-x-auto px-6 pb-2 sm:mx-0 sm:grid sm:grid-cols-2 sm:overflow-visible sm:px-0 md:grid-cols-3 lg:grid-cols-5">
          {shown.map((tc) => (
            <TeacherCard key={tc.id} teacher={tc} className="w-60 shrink-0 sm:w-auto" />
          ))}
        </div>
      </div>
    </div>
  );
}
