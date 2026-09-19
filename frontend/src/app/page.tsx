import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { CalendarCheck, Video, Search } from "lucide-react";
import { listTeachers } from "@/features/teachers/api";
import { listCourseCatalog } from "@/features/courses/api";
import { LanguageTabs } from "@/features/teachers/components/language-tabs";
import { TeacherPhoto } from "@/features/teachers/components/teacher-avatar";
import { CourseCatalogCard } from "@/features/courses/components/course-catalog-card";
import { greetingFor, languageName } from "@/lib/i18n";

const STEPS = [
  { icon: Search, title: "step1Title", body: "step1Body" },
  { icon: CalendarCheck, title: "step2Title", body: "step2Body" },
  { icon: Video, title: "step3Title", body: "step3Body" },
] as const;

/**
 * Udemy's home, with teachers where its courses are: a banner with a white
 * card floating over an image, a tabbed row of cards per language, a row of
 * featured courses, a "become an instructor" split, and a grid of languages.
 */
export default async function HomePage() {
  const [t, tLang, { teachers, facets }, { courses }] = await Promise.all([
    getTranslations("home"),
    getTranslations("languages"),
    listTeachers({ sort: "recommended" }),
    listCourseCatalog({ pageSize: 5, sort: "rating" }).catch(() => ({ courses: [], total: 0 })),
  ]);
  const collage = teachers.slice(0, 4);
  const instructor = teachers[4] ?? teachers[0];

  return (
    <>
      {/* Banner */}
      <section className="mx-auto max-w-[1340px] px-4 pt-6 sm:px-6">
        <div className="relative overflow-hidden bg-muted">
          <div className="grid grid-cols-2 gap-1 lg:ml-auto lg:w-[62%]">
            {collage.map((teacher) => (
              <Link
                key={teacher.id}
                href={`/teachers/${teacher.slug}`}
                className="relative block aspect-[4/3] overflow-hidden lg:aspect-[16/10]"
              >
                <TeacherPhoto
                  src={teacher.avatarUrl}
                  name={teacher.displayName}
                  size={640}
                  sizes="(max-width: 1024px) 50vw, 420px"
                />
              </Link>
            ))}
          </div>

          <div className="bg-background p-6 shadow-card lg:absolute lg:top-1/2 lg:left-16 lg:w-[420px] lg:-translate-y-1/2 lg:p-8">
            <h1 className="font-display text-3xl leading-tight text-foreground lg:text-[2rem]">
              {t("heroTitle")}
            </h1>
            <p className="mt-3 text-base text-foreground/90">{t("heroBody")}</p>
            <div className="mt-5 flex flex-wrap gap-2">
              <Link
                href="/teachers"
                className="inline-flex h-12 items-center rounded-md bg-primary px-4 text-base font-bold text-primary-foreground transition-colors hover:bg-[#8710d8]"
              >
                {t("heroCtaTeachers")}
              </Link>
              <Link
                href="/courses/catalog"
                className="inline-flex h-12 items-center rounded-md border border-foreground bg-background px-4 text-base font-bold text-foreground transition-colors hover:bg-accent"
              >
                {t("heroCtaCourses")}
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* Languages, tabbed */}
      <section className="mx-auto max-w-[1340px] px-4 pt-14 sm:px-6">
        <h2 className="font-display text-2xl sm:text-[2rem]">{t("languagesTitle")}</h2>
        <p className="mt-2 max-w-2xl text-base text-muted-foreground">{t("languagesBody")}</p>
        <div className="mt-6">
          <LanguageTabs teachers={teachers} languages={facets.languages} />
        </div>
      </section>

      {/* Featured courses */}
      {courses.length > 0 && (
        <section className="mx-auto max-w-[1340px] px-4 pt-14 sm:px-6">
          <div className="flex items-end justify-between gap-4">
            <h2 className="font-display text-2xl sm:text-[2rem]">{t("featuredCourses")}</h2>
            <Link
              href="/courses/catalog"
              className="shrink-0 text-sm font-bold text-link underline-offset-2 hover:underline"
            >
              {t("viewAllCourses")}
            </Link>
          </div>
          <div className="mt-6 grid gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
            {courses.map((c) => (
              <CourseCatalogCard key={c.id} course={c} />
            ))}
          </div>
        </section>
      )}

      {/* How it works */}
      <section id="how-it-works" className="mx-auto max-w-[1340px] px-4 pt-14 sm:px-6">
        <h2 className="font-display text-2xl sm:text-[2rem]">{t("howTitle")}</h2>
        <ol className="mt-6 grid gap-6 border-t border-border pt-6 sm:grid-cols-3">
          {STEPS.map(({ icon: Icon, title, body }) => (
            <li key={title} className="flex gap-4">
              <span className="grid size-12 shrink-0 place-items-center rounded-full border border-border text-foreground">
                <Icon className="size-5" />
              </span>
              <div>
                <h3 className="text-base font-bold">{t(title)}</h3>
                <p className="mt-1 text-sm text-muted-foreground">{t(body)}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      {/* Become a teacher */}
      <section id="teach" className="mx-auto max-w-[1340px] px-4 pt-14 sm:px-6">
        <div className="grid items-center gap-8 md:grid-cols-[400px_1fr] lg:gap-16">
          {instructor && (
            <div className="relative mx-auto aspect-[4/5] w-full max-w-[400px] overflow-hidden bg-muted">
              <TeacherPhoto
                src={instructor.avatarUrl}
                name={instructor.displayName}
                size={800}
                sizes="400px"
              />
            </div>
          )}
          <div className="max-w-lg">
            <h2 className="font-display text-2xl sm:text-[2rem]">{t("teachTitle")}</h2>
            <p className="mt-3 text-base text-foreground/90">{t("teachBody")}</p>
            <Link
              href="/signup?role=teacher"
              className="mt-5 inline-flex h-12 items-center rounded-md bg-ink px-4 text-base font-bold text-ink-foreground transition-colors hover:bg-ink/85"
            >
              {t("applyToTeach")}
            </Link>
          </div>
        </div>
      </section>

      {/* Top languages */}
      <section className="mx-auto max-w-[1340px] px-4 pt-14 pb-4 sm:px-6">
        <h2 className="font-display text-2xl sm:text-[2rem]">{t("topLanguages")}</h2>
        <div className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
          {facets.languages.map((l) => (
            <Link
              key={l.code}
              href={`/teachers?lang=${l.code}`}
              className="group block border border-border bg-card transition-shadow hover:shadow-card"
            >
              <div className="grid aspect-square place-items-center bg-muted">
                <span className="font-display text-2xl text-foreground/80">
                  {greetingFor(l.code)}
                </span>
              </div>
              <div className="p-3">
                <p className="text-base font-bold text-foreground group-hover:text-link">
                  {languageName(tLang, l)}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("teacherCount", { count: l.count })}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </section>
    </>
  );
}
