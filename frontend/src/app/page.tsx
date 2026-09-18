import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { CalendarCheck, Video, Search, ShieldCheck, Star, Zap } from "lucide-react";
import { listTeachers } from "@/features/teachers/api";
import { TeacherCard } from "@/features/teachers/components/teacher-card";
import { HeroSearch } from "@/features/teachers/components/hero-search";
import { TeacherPhoto } from "@/features/teachers/components/teacher-avatar";
import { greetingFor, languageName } from "@/lib/i18n";

const TRUST = [
  { icon: ShieldCheck, key: "trustVerified" },
  { icon: Star, key: "trustRating" },
  { icon: Zap, key: "trustReplies" },
  { icon: CalendarCheck, key: "trustPayAfter" },
] as const;

const STEPS = [
  { icon: Search, title: "step1Title", body: "step1Body" },
  { icon: CalendarCheck, title: "step2Title", body: "step2Body" },
  { icon: Video, title: "step3Title", body: "step3Body" },
] as const;

export default async function HomePage() {
  const [t, tLang, { teachers, facets }] = await Promise.all([
    getTranslations("home"),
    getTranslations("languages"),
    listTeachers({ sort: "recommended" }),
  ]);
  const featured = teachers.slice(0, 3);
  const collage = teachers.slice(0, 4);

  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden bg-brand-tint">
        <div className="mx-auto grid max-w-6xl gap-12 px-4 pt-14 pb-16 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:items-center lg:pt-20 lg:pb-24">
          <div>
            <p className="text-sm font-medium text-primary">{t("greetings")}</p>
            <h1 className="mt-4 font-display text-4xl leading-[1.05] text-foreground sm:text-5xl lg:text-[3.5rem]">
              {t("heroTitle")}
            </h1>
            <p className="mt-5 max-w-lg text-lg text-muted-foreground">{t("heroBody")}</p>

            <div className="mt-7 max-w-xl">
              <HeroSearch />
            </div>

            <ul className="mt-6 flex flex-wrap gap-x-5 gap-y-2 text-sm text-muted-foreground">
              {TRUST.map(({ icon: Icon, key }) => (
                <li key={key} className="inline-flex items-center gap-1.5">
                  <Icon className="size-4 text-primary" />
                  {t(key)}
                </li>
              ))}
            </ul>
          </div>

          {/* Photo collage */}
          <div className="relative hidden lg:block">
            <div className="grid grid-cols-2 gap-4">
              {collage.map((teacher, i) => (
                <Link
                  key={teacher.id}
                  href={`/teachers/${teacher.slug}`}
                  className={`group relative block aspect-[5/6] overflow-hidden rounded-3xl shadow-card ring-1 ring-border ${
                    i % 2 === 1 ? "translate-y-6" : ""
                  }`}
                >
                  <TeacherPhoto
                    src={teacher.avatarUrl}
                    name={teacher.displayName}
                    size={480}
                    sizes="260px"
                    className="transition-transform duration-500 group-hover:scale-105"
                  />
                  <span className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent p-3 text-sm font-medium text-white">
                    {teacher.displayName.split(" ")[0]} · {languageName(tLang, teacher.teaches[0])}
                  </span>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Languages */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="font-display text-2xl sm:text-3xl">{t("languagesTitle")}</h2>
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {facets.languages.map((l) => (
            <Link
              key={l.code}
              href={`/teachers?lang=${l.code}`}
              className="group flex items-center justify-between rounded-2xl bg-card p-4 ring-1 ring-border shadow-soft transition-all hover:-translate-y-0.5 hover:shadow-card"
            >
              <div>
                <p className="font-display text-lg text-primary">
                  {greetingFor(l.code)}
                </p>
                <p className="mt-0.5 font-medium text-foreground">{languageName(tLang, l)}</p>
                <p className="text-xs text-muted-foreground">
                  {t("teacherCount", { count: l.count })}
                </p>
              </div>
              <span className="text-muted-foreground transition-transform group-hover:translate-x-0.5">
                →
              </span>
            </Link>
          ))}
        </div>
      </section>

      {/* Featured teachers */}
      <section className="bg-card/60">
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <div className="flex items-end justify-between gap-4">
            <h2 className="font-display text-2xl sm:text-3xl">{t("popularTitle")}</h2>
            <Link
              href="/teachers"
              className="shrink-0 text-sm font-semibold text-primary hover:underline"
            >
              {t("browseAll")}
            </Link>
          </div>
          <div className="mt-6 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {featured.map((teacher) => (
              <TeacherCard key={teacher.id} teacher={teacher} />
            ))}
          </div>
        </div>
      </section>

      {/* How it works */}
      <section id="how-it-works" className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="font-display text-2xl sm:text-3xl">{t("howTitle")}</h2>
        <ol className="mt-8 grid gap-5 sm:grid-cols-3">
          {STEPS.map(({ icon: Icon, title, body }) => (
            <li
              key={title}
              className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft"
            >
              <span className="grid size-11 place-items-center rounded-xl bg-primary/10 text-primary">
                <Icon className="size-5" />
              </span>
              <h3 className="mt-4 font-display text-lg">{t(title)}</h3>
              <p className="mt-1.5 text-sm text-muted-foreground">{t(body)}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* Teach CTA */}
      <section id="teach" className="mx-auto max-w-6xl px-4 pb-16 sm:px-6">
        <div className="flex flex-col gap-5 rounded-3xl bg-primary p-8 text-primary-foreground sm:flex-row sm:items-center sm:justify-between sm:p-10">
          <div className="max-w-lg">
            <h2 className="font-display text-2xl">{t("teachTitle")}</h2>
            <p className="mt-2 text-primary-foreground/80">{t("teachBody")}</p>
          </div>
          <Link
            href="/signup?role=teacher"
            className="inline-flex h-12 shrink-0 items-center justify-center rounded-xl bg-card px-6 font-semibold text-foreground transition-transform hover:scale-[1.02]"
          >
            {t("applyToTeach")}
          </Link>
        </div>
      </section>
    </>
  );
}
