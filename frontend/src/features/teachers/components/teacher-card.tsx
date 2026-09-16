import Image from "next/image";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Play } from "lucide-react";
import type { TeacherSummary } from "@/types/teacher";
import { cn } from "@/lib/utils";
import { flagEmoji } from "@/lib/country";
import { formatMoney } from "@/lib/format";
import { languageName } from "@/lib/i18n";
import { photoUrl } from "./teacher-avatar";
import { Rating } from "./rating";
import { VerifiedBadge } from "./verified-badge";

/**
 * Grid card for the listing and the home page. The whole card is a link; the
 * "Book" affordance is a styled span (no nested interactive elements).
 */
export function TeacherCard({
  teacher,
  className,
}: {
  teacher: TeacherSummary;
  className?: string;
}) {
  const t = useTranslations("teacherCard");
  const tLang = useTranslations("languages");
  const locale = useLocale();
  const href = `/teachers/${teacher.slug}`;
  const alsoCount = teacher.alsoSpeaks.length;

  return (
    <Link
      href={href}
      className={cn(
        "group flex flex-col overflow-hidden rounded-2xl bg-card ring-1 ring-border shadow-card transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        className,
      )}
    >
      <div className="relative aspect-[5/4] overflow-hidden bg-muted">
        <Image
          src={photoUrl(teacher.avatarUrl, 640)}
          alt={teacher.displayName}
          fill
          sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 360px"
          className="object-cover transition-transform duration-500 group-hover:scale-[1.04]"
        />

        <div className="absolute inset-x-0 top-0 flex items-start justify-between p-3">
          {teacher.acceptingStudents ? (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-card/90 px-2 py-1 text-xs font-medium text-foreground shadow-soft backdrop-blur">
              <span className="size-1.5 rounded-full bg-mint" />
              {t("takingStudents")}
            </span>
          ) : (
            <span className="rounded-full bg-card/90 px-2 py-1 text-xs font-medium text-muted-foreground shadow-soft backdrop-blur">
              {t("waitlist")}
            </span>
          )}
          <span className="grid size-9 place-items-center rounded-full bg-card/90 text-foreground shadow-soft backdrop-blur transition-colors group-hover:bg-primary group-hover:text-primary-foreground">
            <Play className="size-4 translate-x-px fill-current" />
          </span>
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-start justify-between gap-2">
          <h3 className="inline-flex items-center gap-1 font-display text-lg leading-tight text-foreground">
            {teacher.displayName}
            {teacher.verified && <VerifiedBadge />}
          </h3>
          <span className="mt-0.5 shrink-0 text-base" title={teacher.countryName}>
            {flagEmoji(teacher.countryCode)}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Rating value={teacher.rating} reviewCount={teacher.reviewCount} variant="pill" />
          <KindTag kind={teacher.kind} label={t(teacher.kind === "professional" ? "professional" : "community")} />
        </div>

        <p className="text-sm text-muted-foreground">
          {t("teaches")}{" "}
          <span className="font-medium text-foreground">
            {teacher.teaches.map((l) => languageName(tLang, l)).join(", ")}
          </span>
          {alsoCount > 0 && ` ${t("more", { count: alsoCount })}`}
        </p>

        <p className="line-clamp-2 text-sm leading-snug text-foreground/90">
          &ldquo;{teacher.headline}&rdquo;
        </p>

        <div className="flex flex-wrap gap-1.5">
          {teacher.focus.slice(0, 2).map((tag) => (
            <span
              key={tag}
              className="rounded-full bg-secondary px-2.5 py-1 text-xs font-medium text-secondary-foreground"
            >
              {tag}
            </span>
          ))}
          {teacher.focus.length > 2 && (
            <span className="rounded-full px-1.5 py-1 text-xs text-muted-foreground">
              +{teacher.focus.length - 2}
            </span>
          )}
        </div>

        <div className="mt-auto flex items-end justify-between gap-2 border-t border-border pt-3">
          <p className="text-sm text-muted-foreground">
            {t.rich("priceFrom", {
              price: formatMoney(teacher.pricePerHour, locale),
              strong: (chunks) => (
                <span className="font-display text-base text-foreground">{chunks}</span>
              ),
            })}
          </p>
          <span className="rounded-lg bg-primary px-3.5 py-2 text-sm font-semibold text-primary-foreground transition-colors group-hover:bg-primary/90">
            {t("viewProfile")}
          </span>
        </div>
      </div>
    </Link>
  );
}

function KindTag({ kind, label }: { kind: TeacherSummary["kind"]; label: string }) {
  if (kind === "professional") {
    return (
      <span className="rounded-full bg-primary/12 px-2 py-0.5 text-xs font-semibold text-primary">
        {label}
      </span>
    );
  }
  return (
    <span className="rounded-full bg-secondary px-2 py-0.5 text-xs font-semibold text-secondary-foreground">
      {label}
    </span>
  );
}
