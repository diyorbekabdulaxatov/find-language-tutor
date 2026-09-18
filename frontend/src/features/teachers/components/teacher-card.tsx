import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import type { TeacherSummary } from "@/types/teacher";
import { cn } from "@/lib/utils";
import { TeacherPhoto } from "@/features/teachers/components/teacher-avatar";
import { formatMoney } from "@/lib/format";
import { languageName } from "@/lib/i18n";
import { Rating } from "./rating";
import { VerifiedBadge } from "./verified-badge";

/**
 * Grid card for the listing and the home page, in Udemy's course-card shape:
 * a flat image, a bold two-line title, a grey by-line, the rating row, the
 * price, and a "Bestseller"-style badge. The whole card is one link.
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
        "group flex flex-col bg-card text-card-foreground outline-none focus-visible:ring-3 focus-visible:ring-ring/40",
        className,
      )}
    >
      <div className="relative aspect-[4/3] overflow-hidden border border-border bg-muted">
        <TeacherPhoto
          src={teacher.avatarUrl}
          name={teacher.displayName}
          size={640}
          sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 300px"
        />
      </div>

      <div className="flex flex-1 flex-col gap-1 pt-2">
        <h3 className="line-clamp-2 text-base leading-tight font-bold text-foreground group-hover:text-link">
          {teacher.displayName}
          {teacher.verified && <VerifiedBadge className="ml-1 inline-block align-text-bottom" />}
        </h3>

        <p className="line-clamp-1 text-xs text-muted-foreground">
          {t("teaches")}{" "}
          {teacher.teaches.map((l) => languageName(tLang, l)).join(", ")}
          {alsoCount > 0 && ` ${t("more", { count: alsoCount })}`}
        </p>

        <p className="line-clamp-2 text-xs leading-snug text-muted-foreground">{teacher.headline}</p>

        <Rating value={teacher.rating} reviewCount={teacher.reviewCount} variant="pill" />

        <p className="text-base font-bold text-foreground">
          {formatMoney(teacher.pricePerHour, locale)}
          <span className="text-xs font-normal text-muted-foreground"> {t("perHour")}</span>
        </p>

        <div className="mt-0.5 flex flex-wrap gap-1.5">
          {teacher.kind === "professional" && (
            <span className="bg-[#eceb98] px-2 py-0.5 text-xs font-bold text-[#3d3c0a]">
              {t("professional")}
            </span>
          )}
          {!teacher.acceptingStudents && (
            <span className="bg-muted px-2 py-0.5 text-xs font-bold text-muted-foreground">
              {t("waitlist")}
            </span>
          )}
        </div>
      </div>
    </Link>
  );
}
