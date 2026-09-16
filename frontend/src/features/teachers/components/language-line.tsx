import { useLocale, useTranslations } from "next-intl";
import type { SpokenLanguage } from "@/types/teacher";
import { languageName } from "@/lib/i18n";

export function LanguageLine({
  teaches,
  alsoSpeaks,
  className,
}: {
  teaches: SpokenLanguage[];
  alsoSpeaks: SpokenLanguage[];
  className?: string;
}) {
  const t = useTranslations("profile");
  const tLang = useTranslations("languages");
  const locale = useLocale();
  // "English and Portuguese" / "английский, русский и узбекский" — per-locale
  // list punctuation without hand-rolling it.
  const list = new Intl.ListFormat(locale, { style: "long", type: "conjunction" });

  const taught = list.format(teaches.map((l) => languageName(tLang, l)));
  const also = alsoSpeaks.length ? list.format(alsoSpeaks.map((l) => languageName(tLang, l))) : "";

  return (
    <p className={className}>
      {t("teaches", { languages: taught })}
      {also && <span className="text-muted-foreground"> {t("alsoSpeaks", { languages: also })}</span>}
    </p>
  );
}
