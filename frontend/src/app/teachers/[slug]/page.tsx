import { cache } from "react";
import Link from "next/link";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";

import { getTeacherBySlug, listTeacherSlugs } from "@/features/teachers/api";
import { Badge } from "@/components/ui/badge";
import { Rating } from "@/features/teachers/components/rating";
import { LanguageLine } from "@/features/teachers/components/language-line";
import { LocalTime } from "@/features/teachers/components/local-time";
import { IntroVideo } from "@/features/teachers/components/intro-video";
import { BookingPanel } from "@/features/teachers/components/booking-panel";
import { TeacherAvatar } from "@/features/teachers/components/teacher-avatar";
import { greetingFor, localTimeIn } from "@/lib/i18n";

/**
 * `cache` memoises the loader for one request, so `generateMetadata` and the
 * page component share a single lookup instead of fetching twice.
 */
const loadTeacher = cache(getTeacherBySlug);

/**
 * Prerender every profile at build time. Combined with the default static
 * rendering, this makes profile pages fast and crawlable (the SEO requirement).
 */
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
      images: [teacher.videoThumbnailUrl],
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
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:py-12">
      <Link
        href="/teachers"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        All teachers
      </Link>

      <div className="mt-6 grid gap-x-12 gap-y-10 lg:grid-cols-[1fr_340px]">
        {/* Masthead + intro — column 1, row 1 */}
        <div className="lg:col-start-1 lg:row-start-1">
          <div className="flex items-start gap-4">
            <TeacherAvatar
              src={teacher.avatarUrl}
              name={teacher.displayName}
              size={64}
            />
            <div>
              <p className="font-display text-lg italic text-[var(--color-link)]">
                {greetingFor(primaryLanguage.code)}.
              </p>
              <h1 className="mt-0.5 font-display text-3xl font-medium tracking-tight sm:text-4xl">
                {teacher.displayName}
              </h1>
              <p className="mt-1 text-muted-foreground">{kindLabel}</p>
            </div>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-1 text-sm">
            <span className="text-muted-foreground">
              {teacher.city}, {teacher.countryName}
            </span>
            <LocalTime
              timezone={teacher.timezone}
              city={teacher.city}
              initial={localTimeIn(teacher.timezone)}
            />
          </div>

          <p className="mt-6 max-w-[42ch] font-display text-xl italic leading-snug text-foreground/90">
            &ldquo;{teacher.headline}&rdquo;
          </p>

          <div className="mt-6">
            <IntroVideo poster={teacher.videoThumbnailUrl} name={teacher.displayName} />
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 text-sm">
            <Rating value={teacher.rating} reviewCount={teacher.reviewCount} />
            <span className="text-muted-foreground">
              {teacher.studentCount} active students
            </span>
          </div>

          <LanguageLine
            teaches={teacher.teaches}
            alsoSpeaks={teacher.alsoSpeaks}
            className="mt-3 text-sm"
          />

          <div className="mt-4 flex flex-wrap gap-2">
            {teacher.focus.map((tag) => (
              <Badge key={tag} variant="outline" className="font-normal">
                {tag}
              </Badge>
            ))}
          </div>
        </div>

        {/* Booking panel — column 2, spanning both rows, sticky */}
        <aside className="lg:col-start-2 lg:row-start-1 lg:row-span-2 lg:sticky lg:top-20 lg:self-start">
          <BookingPanel teacher={teacher} />
        </aside>

        {/* Long-form content — column 1, row 2 */}
        <div className="space-y-10 lg:col-start-1 lg:row-start-2">
          <Section title="About">
            <Prose text={teacher.about} />
          </Section>

          <Section title="How I teach">
            <Prose text={teacher.teachingStyle} />
          </Section>

          <Section title="Experience">
            <ul className="space-y-3">
              {teacher.experience.map((item) => (
                <li
                  key={`${item.title}-${item.period}`}
                  className="grid gap-x-4 gap-y-0.5 text-sm sm:grid-cols-[8rem_1fr]"
                >
                  <span className="text-muted-foreground">{item.period}</span>
                  <span>
                    {item.title} — {item.org}
                  </span>
                </li>
              ))}
            </ul>
          </Section>
        </div>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="font-display text-xl font-medium">{title}</h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}

/** Renders \n\n-separated plain text as paragraphs at a readable measure. */
function Prose({ text }: { text: string }) {
  return (
    <div className="max-w-[62ch] space-y-4 leading-relaxed text-foreground/90">
      {text.split("\n\n").map((para, i) => (
        <p key={i}>{para}</p>
      ))}
    </div>
  );
}
