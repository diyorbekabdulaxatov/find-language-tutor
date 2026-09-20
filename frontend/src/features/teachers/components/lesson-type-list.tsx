import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";
import { Sparkles } from "lucide-react";
import { formatMoney } from "@/lib/format";
import type { LessonType } from "@/features/teachers/api";

/**
 * italki's "what you can book" block: one card per offering, each with its own
 * description and a price per length. The trial sorts first and is badged. Each
 * length is a link straight into the checkout with that offering preselected,
 * so the student never picks a price twice.
 */
export async function LessonTypeList({
  slug,
  lessonTypes,
  accepting,
}: {
  slug: string;
  lessonTypes: LessonType[];
  accepting: boolean;
}) {
  const [t, locale] = await Promise.all([getTranslations("lessonTypes"), getLocale()]);
  if (lessonTypes.length === 0) return null;

  return (
    <section className="mt-10">
      <h2 className="font-display text-xl">{t("title")}</h2>
      <p className="mt-1 text-sm text-muted-foreground">{t("intro")}</p>

      <ul className="mt-4 flex flex-col gap-3">
        {lessonTypes.map((lt) => (
          <li key={lt.id} className="border border-border bg-card p-5">
            <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
              <h3 className="inline-flex items-center gap-2 font-display text-lg">
                {lt.title}
                {lt.isTrial && (
                  <span className="inline-flex items-center gap-1 rounded-sm bg-accent px-1.5 py-0.5 text-xs font-bold text-accent-foreground">
                    <Sparkles className="size-3" />
                    {t("trialBadge")}
                  </span>
                )}
              </h3>
              <span className="text-sm text-muted-foreground">
                {t("from", { price: formatMoney(lt.from, locale) })}
              </span>
            </div>

            {lt.description && (
              <p className="mt-2 text-sm text-muted-foreground">{lt.description}</p>
            )}

            <div className="mt-4 flex flex-wrap gap-2">
              {lt.prices.map((p) =>
                accepting ? (
                  <Link
                    key={p.durationMinutes}
                    href={`/teachers/${slug}/book?lesson=${lt.id}&duration=${p.durationMinutes}`}
                    className="flex h-11 items-center gap-2 rounded-md border border-foreground bg-background px-3 text-sm font-bold text-foreground transition-colors hover:bg-accent"
                  >
                    <span>{t("minutes", { count: p.durationMinutes })}</span>
                    <span className="text-muted-foreground">·</span>
                    <span>{formatMoney(p.price, locale)}</span>
                  </Link>
                ) : (
                  <span
                    key={p.durationMinutes}
                    className="flex h-11 items-center gap-2 rounded-md border border-border px-3 text-sm font-bold text-muted-foreground"
                  >
                    <span>{t("minutes", { count: p.durationMinutes })}</span>
                    <span>·</span>
                    <span>{formatMoney(p.price, locale)}</span>
                  </span>
                ),
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
