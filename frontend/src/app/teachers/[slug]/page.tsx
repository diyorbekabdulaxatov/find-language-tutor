import { cache } from "react";
import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";

import { getTeacherBySlug, listTeacherSlugs } from "@/features/teachers/api";
import { Rating } from "@/features/teachers/components/rating";
import { LanguageLine } from "@/features/teachers/components/language-line";
import { LocalTime } from "@/features/teachers/components/local-time";
import { IntroVideo } from "@/features/teachers/components/intro-video";
import { BookingPanel } from "@/features/teachers/components/booking-panel";
import { ReviewsSection } from "@/features/reviews/components/reviews-section";
import { photoUrl } from "@/features/teachers/components/teacher-avatar";
import { greetingFor, localTimeIn } from "@/lib/i18n";
import { flagEmoji } from "@/lib/country";

/**
 * `cache` memoises the loader for one request, so `generateMetadata` and the
 * page component share a single lookup instead of fetching twice.
 */
const loadTeacher = cache(getTeacherBySlug);

/**
 * Prerender every profile the backend knows about at build time. Combined with
 * the default static rendering, this makes profile pages fast and crawlable
 * (the SEO requirement).
 *
 * `dynamicParams` stays true so a teacher added after the last build still
 * renders (on demand, then cached) instead of 404-ing.
 */
export const dynamicParams = true;

export async function generateStaticParams() {
  const slugs = await listTeacherSlugs();
  return slugs.map((slug) => ({ slug }));
}

export async function generateMetadata({
  params,
}: PageProps<"/teachers/[slug]">): Promise<Metadata> {
  const { slug } = await params;
  const teacher = await loadTeacher(slug);
  if (!teacher) return {};

  const subject = teacher.teaches.map((l) => l.name).join(" & ");
  return {
    title: `${teacher.displayName} — ${subject} teacher`,
    description: teacher.headline,
    openGraph: {
      title: `${teacher.displayName} · ${subject} on findtutor`,
      description: teacher.headline,
      images: [teacher.avatarUrl],
    },
  };
}

export default async function TeacherProfilePage({
  params,
}: PageProps<"/teachers/[slug]">) {
  const { slug } = await params;
  const teacher = await loadTeacher(slug);
  if (!teacher) notFound();

  const primaryLanguage = teacher.teaches[0];
  const kindLabel =
    teacher.kind === "professional"
      ? `Professional teacher of ${primaryLanguage.name}`
      : `Community tutor of ${primaryLanguage.name}`;

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-10">
      <Link
        href="/teachers"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        All teachers
      </Link>

      <div className="mt-5 grid gap-6 lg:grid-cols-[1fr_340px]">
        {/* Header card — column 1, row 1 */}
        <div className="rounded-2xl bg-card p-5 ring-1 ring-border shadow-card sm:p-6 lg:col-start-1 lg:row-start-1">
          <div className="grid gap-5 sm:grid-cols-[220px_1fr] sm:gap-6">
            <IntroVideo
              poster={photoUrl(teacher.avatarUrl, 640)}
              name={teacher.displayName}
              className="aspect-[4/5] w-full"
            />

            <div>
              <p className="font-display text-lg text-primary">
                {greetingFor(primaryLanguage.code)}.
              </p>
              <h1 className="mt-1 flex flex-wrap items-center gap-2 font-display text-3xl tracking-tight sm:text-[2rem]">
                {teacher.displayName}
                <span className="text-2xl" title={teacher.countryName}>
                  {flagEmoji(teacher.countryCode)}
                </span>
              </h1>
              <p className="mt-1 text-muted-foreground">{kindLabel}</p>

              <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm">
                <Rating value={teacher.rating} reviewCount={teacher.reviewCount} />
                <span className="text-muted-foreground">
                  {teacher.studentCount} active students
                </span>
              </div>

              <div className="mt-2 text-sm">
                <LocalTime
                  timezone={teacher.timezone}
                  city={teacher.city}
                  initial={localTimeIn(teacher.timezone)}
                />
              </div>

              <LanguageLine
                teaches={teacher.teaches}
                alsoSpeaks={teacher.alsoSpeaks}
                className="mt-3 text-sm"
              />

              <div className="mt-4 flex flex-wrap gap-1.5">
                {teacher.focus.map((tag) => (
                  <span
                    key={tag}
                    className="rounded-full bg-secondary px-2.5 py-1 text-xs font-medium text-secondary-foreground"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          </div>

          <p className="mt-5 border-t border-border pt-5 font-display text-xl leading-snug text-foreground">
            &ldquo;{teacher.headline}&rdquo;
          </p>
        </div>

        {/* Booking panel — column 2, spanning both rows, sticky */}
        <aside className="lg:sticky lg:top-20 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:self-start">
          <BookingPanel teacher={teacher} />
        </aside>

        {/* Long-form content — column 1, row 2 */}
        <div className="space-y-6 lg:col-start-1 lg:row-start-2">
          <Card title="About">
            <Prose text={teacher.about} />
          </Card>

          <Card title="How I teach">
            <Prose text={teacher.teachingStyle} />
          </Card>

          <Card title="Experience">
            <ul className="space-y-4">
              {teacher.experience.map((item) => (
                <li
                  key={`${item.title}-${item.period}`}
                  className="grid gap-x-4 gap-y-0.5 text-sm sm:grid-cols-[9rem_1fr]"
                >
                  <span className="font-medium text-muted-foreground">
                    {item.period}
                  </span>
                  <span className="text-foreground">
                    {item.title} — {item.org}
                  </span>
                </li>
              ))}
            </ul>
          </Card>

          <ReviewsSection slug={slug} reviewCount={teacher.reviewCount} />
        </div>
      </div>
    </div>
  );
}

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
      <h2 className="font-display text-xl">{title}</h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}

/** Renders \n\n-separated plain text as paragraphs at a readable measure. */
function Prose({ text }: { text: string }) {
  return (
    <div className="max-w-[64ch] space-y-4 leading-relaxed text-foreground/90">
      {text.split("\n\n").map((para, i) => (
        <p key={i}>{para}</p>
      ))}
    </div>
  );
}
