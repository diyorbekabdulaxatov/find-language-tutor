import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { CalendarCheck, Clock, ShieldCheck } from "lucide-react";
import type { TeacherProfile } from "@/types/teacher";
import { formatMoney } from "@/lib/format";

/**
 * The price + book actions. Sticky on desktop (positioned by the profile page),
 * a plain block on mobile. "Book" links into the /teachers/[slug]/book flow.
 */
export function BookingPanel({ teacher }: { teacher: TeacherProfile }) {
  const t = useTranslations("bookingPanel");
  const locale = useLocale();

  const hours = teacher.responseTimeHours;
  const responseTime =
    hours < 1
      ? t("underAnHour")
      : hours < 24
        ? t("hours", { count: Math.round(hours) })
        : t("days", { count: Math.round(hours / 24) });

  return (
    <div className="bg-card p-6 text-card-foreground shadow-card lg:border lg:border-border">
      <div className="flex items-end gap-1.5">
        <span className="font-display text-[2rem] leading-none text-foreground">
          {formatMoney(teacher.pricePerHour, locale)}
        </span>
        <span className="pb-0.5 text-sm text-muted-foreground">{t("per60")}</span>
      </div>

      {teacher.trialPrice && (
        <p className="mt-2 text-sm text-muted-foreground">
          {t("trial", { price: formatMoney(teacher.trialPrice, locale) })}
        </p>
      )}

      <div className="mt-4 space-y-2">
        {teacher.acceptingStudents ? (
          <>
            <Link
              href={`/teachers/${teacher.slug}/book`}
              className="flex h-12 w-full items-center justify-center rounded-md bg-primary text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
            >
              {t("bookLesson")}
            </Link>
            {teacher.trialPrice && (
              <Link
                href={`/teachers/${teacher.slug}/book?trial=1`}
                className="flex h-12 w-full items-center justify-center rounded-md border border-foreground bg-background text-base font-bold text-foreground transition-colors hover:bg-accent"
              >
                {t("bookTrial")}
              </Link>
            )}
          </>
        ) : (
          <button
            disabled
            className="h-12 w-full cursor-not-allowed rounded-md bg-muted text-base font-bold text-muted-foreground"
          >
            {t("notTaking")}
          </button>
        )}
      </div>

      <p className="mt-3 text-center text-xs text-muted-foreground">{t("chargedNote")}</p>

      <dl className="mt-5 space-y-2 border-t border-border pt-4 text-sm">
        <Row icon={Clock} label={t("repliesIn")}>
          {responseTime}
        </Row>
        <Row icon={CalendarCheck} label={t("lessonsTaught")}>
          {teacher.lessonsCompleted.toLocaleString(locale)}
        </Row>
        <Row icon={ShieldCheck} label={t("activeStudents")}>
          {teacher.studentCount}
        </Row>
      </dl>
    </div>
  );
}

function Row({
  icon: Icon,
  label,
  children,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between">
      <dt className="inline-flex items-center gap-2 text-muted-foreground">
        <Icon className="size-4" />
        {label}
      </dt>
      <dd className="font-bold text-foreground">{children}</dd>
    </div>
  );
}
