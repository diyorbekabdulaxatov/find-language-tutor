import Link from "next/link";
import Image from "next/image";
import { CalendarCheck, Video, Search, ShieldCheck, Star, Zap } from "lucide-react";
import { listTeachers } from "@/features/teachers/api";
import { TeacherCard } from "@/features/teachers/components/teacher-card";
import { HeroSearch } from "@/features/teachers/components/hero-search";
import { photoUrl } from "@/features/teachers/components/teacher-avatar";
import { greetingFor } from "@/lib/i18n";

const TRUST = [
  { icon: ShieldCheck, label: "Verified teachers" },
  { icon: Star, label: "4.8 average rating" },
  { icon: Zap, label: "Most reply within hours" },
  { icon: CalendarCheck, label: "Pay after the lesson" },
];

const STEPS = [
  {
    icon: Search,
    title: "Find a teacher",
    body: "Filter by language, price, and focus. Watch a one-minute intro before you commit to anything.",
  },
  {
    icon: CalendarCheck,
    title: "Book a time that works",
    body: "Pick a slot from the teacher's calendar. Times show in your timezone — no maths to do.",
  },
  {
    icon: Video,
    title: "Meet on video",
    body: "You get a video link and a reminder. Pay in so'm after the lesson is confirmed, not before.",
  },
];

export default async function HomePage() {
  const { teachers, facets } = await listTeachers({ sort: "recommended" });
  const featured = teachers.slice(0, 3);
  const collage = teachers.slice(0, 4);

  return (
    <>
      {/* Hero */}
      <section className="relative overflow-hidden bg-brand-tint">
        <div className="mx-auto grid max-w-6xl gap-12 px-4 pt-14 pb-16 sm:px-6 lg:grid-cols-[1.05fr_0.95fr] lg:items-center lg:pt-20 lg:pb-24">
          <div>
            <p className="text-sm font-medium text-primary">
              Salom · Hello · Привет · 안녕하세요 · Merhaba
            </p>
            <h1 className="mt-4 font-display text-4xl leading-[1.05] text-foreground sm:text-5xl lg:text-[3.5rem]">
              Find a language teacher who gets you talking
            </h1>
            <p className="mt-5 max-w-lg text-lg text-muted-foreground">
              One-on-one video lessons with English, Russian, and more, from
              teachers across Uzbekistan and beyond. Pay per lesson in so&rsquo;m.
            </p>

            <div className="mt-7 max-w-xl">
              <HeroSearch />
            </div>

            <ul className="mt-6 flex flex-wrap gap-x-5 gap-y-2 text-sm text-muted-foreground">
              {TRUST.map(({ icon: Icon, label }) => (
                <li key={label} className="inline-flex items-center gap-1.5">
                  <Icon className="size-4 text-primary" />
                  {label}
                </li>
              ))}
            </ul>
          </div>

          {/* Photo collage */}
          <div className="relative hidden lg:block">
            <div className="grid grid-cols-2 gap-4">
              {collage.map((t, i) => (
                <Link
                  key={t.id}
                  href={`/teachers/${t.slug}`}
                  className={`group relative block aspect-[5/6] overflow-hidden rounded-3xl shadow-card ring-1 ring-border ${
                    i % 2 === 1 ? "translate-y-6" : ""
                  }`}
                >
                  <Image
                    src={photoUrl(t.avatarUrl, 480)}
                    alt={t.displayName}
                    fill
                    sizes="260px"
                    className="object-cover transition-transform duration-500 group-hover:scale-105"
                  />
                  <span className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/60 to-transparent p-3 text-sm font-medium text-white">
                    {t.displayName.split(" ")[0]} · {t.teaches[0].name}
                  </span>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* Languages */}
      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <h2 className="font-display text-2xl sm:text-3xl">Languages you can learn</h2>
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
                <p className="mt-0.5 font-medium text-foreground">{l.name}</p>
                <p className="text-xs text-muted-foreground">
                  {l.count} {l.count === 1 ? "teacher" : "teachers"}
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
            <h2 className="font-display text-2xl sm:text-3xl">
              Popular teachers this week
            </h2>
            <Link
              href="/teachers"
              className="shrink-0 text-sm font-semibold text-primary hover:underline"
            >
              Browse all →
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
        <h2 className="font-display text-2xl sm:text-3xl">How lessons work</h2>
        <ol className="mt-8 grid gap-5 sm:grid-cols-3">
          {STEPS.map(({ icon: Icon, title, body }) => (
            <li
              key={title}
              className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft"
            >
              <span className="grid size-11 place-items-center rounded-xl bg-primary/10 text-primary">
                <Icon className="size-5" />
              </span>
              <h3 className="mt-4 font-display text-lg">{title}</h3>
              <p className="mt-1.5 text-sm text-muted-foreground">{body}</p>
            </li>
          ))}
        </ol>
      </section>

      {/* Teach CTA */}
      <section id="teach" className="mx-auto max-w-6xl px-4 pb-16 sm:px-6">
        <div className="flex flex-col gap-5 rounded-3xl bg-primary p-8 text-primary-foreground sm:flex-row sm:items-center sm:justify-between sm:p-10">
          <div className="max-w-lg">
            <h2 className="font-display text-2xl">Teach on FindTutor</h2>
            <p className="mt-2 text-primary-foreground/80">
              Set your own rates and hours. We handle scheduling, payments, and
              payouts to your card in so&rsquo;m.
            </p>
          </div>
          <Link
            href="/signup?role=teacher"
            className="inline-flex h-12 shrink-0 items-center justify-center rounded-xl bg-card px-6 font-semibold text-foreground transition-transform hover:scale-[1.02]"
          >
            Apply to teach
          </Link>
        </div>
      </section>
    </>
  );
}
