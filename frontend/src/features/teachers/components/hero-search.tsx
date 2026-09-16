"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

/** Languages we currently have teachers for. Mirrors the catalog. */
const LANGUAGES = ["en", "ru", "uz", "de", "ko", "tr"] as const;

export function HeroSearch() {
  const t = useTranslations("heroSearch");
  const tLang = useTranslations("languages");
  const router = useRouter();
  const [q, setQ] = useState("");
  const [lang, setLang] = useState("any");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const params = new URLSearchParams();
    if (q.trim()) params.set("q", q.trim());
    if (lang !== "any") params.set("lang", lang);
    router.push(params.toString() ? `/teachers?${params}` : "/teachers");
  }

  return (
    <form
      onSubmit={submit}
      className="flex flex-col gap-2 rounded-2xl bg-card p-2 shadow-lift ring-1 ring-border sm:flex-row sm:items-center"
    >
      <div className="flex flex-1 items-center gap-2 px-3">
        <Search className="size-5 shrink-0 text-muted-foreground" />
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={t("placeholder")}
          className="h-11 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          aria-label={t("ariaLabel")}
        />
      </div>

      <div className="flex items-center gap-2">
        <Select value={lang} onValueChange={setLang}>
          <SelectTrigger className="h-11 w-[150px] rounded-xl border-border">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="any">{t("anyLanguage")}</SelectItem>
            {LANGUAGES.map((code) => (
              <SelectItem key={code} value={code}>
                {tLang(code)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <button
          type="submit"
          className="h-11 rounded-xl bg-primary px-5 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {t("search")}
        </button>
      </div>
    </form>
  );
}
