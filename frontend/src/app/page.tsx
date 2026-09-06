import Link from "next/link";
import { Button } from "@/components/ui/button";
import { listTeachers } from "@/features/teachers/api";
import { TeacherListItem } from "@/features/teachers/components/teacher-list-item";
import { TeacherAvatar } from "@/features/teachers/components/teacher-avatar";
import { Rating } from "@/features/teachers/components/rating";
import { formatMoney } from "@/lib/format";

const GREETINGS = ["Salom", "Hello", "Привет", "안녕하세요", "Merhaba", "Hallo"];

const QUICK_LANGUAGES = [
  { code: "en", label: "English" },
  { code: "ru", label: "Russian" },
  { code: "de", label: "German" },
  { code: "ko", label: "Korean" },
  { code: "tr", label: "Turkish" },
];

const STEPS = [
  {
    title: "Find a teacher",
    body: "Filter by language, price, and focus. Watch a one-minute intro before you commit to anything.",
  },
  {
    title: "Book a time that works",
    body: "Pick a slot from the teacher's calendar. Times show in your timezone, so there's no maths to do.",
  },
  {
    title: "Meet on video",
    body: "You get a video link and a reminder. Pay in so'm after the lesson is confirmed — not before.",
  },
];

export default async function HomePage() {
  const { teachers } = await listTeachers({ sort: "recommended" });
  const spotlight = teachers[0];
  // Skip the spotlight teacher so the list below doesn't repeat them.
  const featured = teachers.slice(1, 4);

  return (
    <>
      <section className="mx-auto max-w-6xl px-4 pt-16 pb-14 sm:px-6 lg:grid lg:grid-cols-[1fr_360px] lg:items-center lg:gap-16 lg:pt-24">
        <div>
          <p className="font-display text-2xl leading-relaxed text-[var(--color-link)] sm:text-3xl">
            {GREETINGS.map((g, i) => (
              <span key={g}>
                {g}.{i < GREETINGS.length - 1 ? " " : ""}
              </span>
            ))}
          </p>

          <h1 className="mt-4 max-w-2xl font-display text-4xl font-medium leading-[1.1] tracking-tight sm:text-5xl">
            Learn a language with a teacher who gets you talking
          </h1>
          <p className="mt-5 max-w-xl text-lg text-muted-foreground">
            Book one-on-one video lessons with English, Russian, and other
            language teachers across Uzbekistan and beyond. Pay per lesson in
            so&rsquo;m, cancel free up to 12 hours before.
          </p>

          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Button asChild className="h-11 px-5 text-[0.95rem]">
              <Link href="/teachers">Find a teacher</Link>
            </Button>
            <Button variant="ghost" asChild className="h-11 px-4 text-[0.95rem]">
              <Link href="#how-it-works">See how it works</Link>
            </Button>
          </div>

          <div className="mt-8 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground">
            <span>Popular:</span>
            {QUICK_LANGUAGES.map((lang) => (
              <Link
                key={lang.code}
                href={`/teachers?lang=${lang.code}`}
                className="rounded-full px-2 py-0.5 text-foreground underline-offset-4 hover:bg-secondary hover:underline"
              >
                {lang.label}
              </Link>
            ))}
          </div>
        </div>

        {spotlight && (
          <Link
            href={`/teachers/${spotlight.slug}`}
            className="group mt-12 block rounded-xl bg-card p-5 ring-1 ring-foreground/10 transition-shadow hover:shadow-md lg:mt-0"
          >
            <p className="text-xs text-muted-foreground">Booked most this week</p>
            <div className="mt-3 flex items-center gap-3">
              <TeacherAvatar src={spotlight.avatarUrl} name={spotlight.displayName} size={44} />
              <div>
                <p className="font-semibold leading-tight group-hover:underline">
                  {spotlight.displayName}
                </p>
                <p className="text-sm text-muted-foreground">
                  {spotlight.teaches.map((l) => l.name).join(" & ")} teacher in{" "}
                  {spotlight.city}
                </p>
              </div>
            </div>
            <p className="mt-3 font-display text-lg italic leading-snug text-foreground/90">
              &ldquo;{spotlight.headline}&rdquo;
            </p>
            <div className="mt-3 flex items-center justify-between text-sm">
              <Rating value={spotlight.rating} reviewCount={spotlight.reviewCount} />
              <span className="font-medium">
                {formatMoney(spotlight.pricePerHour)}
                <span className="text-muted-foreground"> / hr</span>
              </span>
            </div>
          </Link>
        )}
      </section>

      <section
        id="how-it-works"
        className="border-y border-border bg-secondary/40"
      >
        <div className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
          <h2 className="font-display text-2xl font-medium sm:text-3xl">
            How lessons work
          </h2>
          <ol className="mt-8 grid gap-8 sm:grid-cols-3">
            {STEPS.map((step, i) => (
              <li key={step.title}>
                <span className="font-display text-2xl text-muted-foreground">
                  {i + 1}
                </span>
                <h3 className="mt-1 text-base font-semibold">{step.title}</h3>
                <p className="mt-1.5 text-sm text-muted-foreground">{step.body}</p>
              </li>
            ))}
          </ol>
        </div>
      </section>

      <section className="mx-auto max-w-6xl px-4 py-16 sm:px-6">
        <div className="flex items-end justify-between gap-4">
          <h2 className="font-display text-2xl font-medium sm:text-3xl">
            Teachers people are booking
          </h2>
          <Link
            href="/teachers"
            className="shrink-0 text-sm text-[var(--color-link)] underline-offset-4 hover:underline"
          >
            See all
          </Link>
        </div>

        <div className="mt-6">
          {featured.map((teacher) => (
            <TeacherListItem key={teacher.id} teacher={teacher} />
          ))}
        </div>
      </section>

      <section id="teach" className="border-t border-border">
        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-4 py-16 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="max-w-lg">
            <h2 className="font-display text-2xl font-medium">
              Teach on findtutor
            </h2>
            <p className="mt-2 text-muted-foreground">
              Set your own rates and hours. We handle scheduling, payments, and
              payouts to your card in so&rsquo;m.
            </p>
          </div>
          <Button variant="outline" asChild className="h-11 shrink-0 px-5">
            <Link href="/signup?role=teacher">Apply to teach</Link>
          </Button>
        </div>
      </section>
    </>
  );
}
