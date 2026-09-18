import { cache } from "react";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";

import { getTeacherBySlug, listTeacherSlugs } from "@/features/teachers/api";
import { Stars } from "@/features/reviews/components/star-rating";
import { LanguageLine } from "@/features/teachers/components/language-line";
import { LocalTime } from "@/features/teachers/components/local-time";
import { IntroVideo } from "@/features/teachers/components/intro-video";
import { BookingPanel } from "@/features/teachers/components/booking-panel";
import { ReviewsSection } from "@/features/reviews/components/reviews-section";
import { VerifiedBadge } from "@/features/teachers/components/verified-badge";
import { languageName, localTimeIn } from "@/lib/i18n";
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

  const [t, tLang] = await Promise.all([getTranslations("profile"), getTranslations("languages")]);
  const subject = teacher.teaches.map((l) => languageName(tLang, l)).join(" & ");
  return {
    title: t("metaTitle", { name: teacher.displayName, subject }),
    description: teacher.headline,
    openGraph: {
      title: t("ogTitle", { name: teacher.displayName, subject }),
      description: teacher.headline,
      images: teacher.avatarUrl ? [teacher.avatarUrl] : [],
    },
  };
}

export default async function TeacherProfilePage({
  params,
}: PageProps<"/teachers/[slug]">) {
  const { slug } = await params;
  const teacher = await loadTeacher(slug);
  if (!teacher) notFound();

  const [t, tLang, locale] = await Promise.all([
    getTranslations("profile"),
    getTranslations("languages"),
    getLocale(),
  ]);
  const primaryLanguage = teacher.teaches[0];
  const kindLabel = t(teacher.kind === "professional" ? "professionalOf" : "communityOf", {
    language: languageName(tLang, primaryLanguage),
  });

  return (
    <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6 lg:py-12">
      <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-16">
        {/* Column 1: the instructor page body. */}
        <div className="max-w-[760px]">
          <p className="text-sm font-bold text-muted-foreground">{kindLabel}</p>
          <h1 className="mt-1 flex flex-wrap items-center gap-2 font-display text-3xl sm:text-[2.5rem]">
            {teacher.displayName}
            {teacher.verified && <VerifiedBadge className="[&_svg]:size-6" />}
            <span className="text-2xl" title={teacher.countryName}>
              {flagEmoji(teacher.countryCode)}
            </span>
          </h1>
          <p className="mt-2 text-lg font-bold text-foreground/90">{teacher.headline}</p>

          <div className="mt-6 flex flex-wrap gap-10">
            <Stat label={t("activeStudentsLabel")} value={teacher.studentCount.toLocaleString(locale)} />
            <Stat label={t("reviewsLabel")} value={teacher.reviewCount.toLocaleString(locale)} />
            <Stat label={t("ratingLabel")} value={teacher.rating.toFixed(1)}>
              <Stars value={teacher.rating} className="mt-1 [&_svg]:size-3.5" />
            </Stat>
          </div>

          <div className="mt-6 space-y-1.5 text-sm">
            <LocalTime
              timezone={teacher.timezone}
              city={teacher.city}
              initial={localTimeIn(teacher.timezone, new Date(), locale)}
            />
            <LanguageLine teaches={teacher.teaches} alsoSpeaks={teacher.alsoSpeaks} />
          </div>

          <div className="mt-4 flex flex-wrap gap-1.5">
            {teacher.focus.map((tag) => (
              <span key={tag} className="border border-border px-2 py-0.5 text-xs text-muted-foreground">
                {tag}
              </span>
            ))}
          </div>

          {/* Photo + intro on phones, above the fold. */}
          <div className="mt-8 lg:hidden">
            <IntroVideo
              poster={teacher.avatarUrl}
              videoUrl={teacher.introVideoUrl}
              name={teacher.displayName}
              className="aspect-[4/3] w-full"
            />
            <div className="mt-6">
              <BookingPanel teacher={teacher} />
            </div>
          </div>

          <Section title={t("about")}>
            <Prose text={teacher.about} />
          </Section>

          <Section title={t("howITeach")}>
            <Prose text={teacher.teachingStyle} />
          </Section>

          <Section title={t("experience")}>
            <ul className="space-y-3">
              {teacher.experience.map((item) => (
                <li
                  key={`${item.title}-${item.period}`}
                  className="grid gap-x-4 gap-y-0.5 text-sm sm:grid-cols-[9rem_1fr]"
                >
                  <span className="font-bold text-muted-foreground">{item.period}</span>
                  <span className="text-foreground">
                    {item.title} — {item.org}
                  </span>
                </li>
              ))}
            </ul>
          </Section>

          <div className="mt-10">
            <ReviewsSection slug={slug} reviewCount={teacher.reviewCount} />
          </div>
        </div>

        {/* Column 2: photo / intro video, then the sticky booking card. */}
        <aside className="hidden lg:block">
          <div className="sticky top-[88px] space-y-6">
            <IntroVideo
              poster={teacher.avatarUrl}
              videoUrl={teacher.introVideoUrl}
              name={teacher.displayName}
              className="aspect-square w-full"
            />
            <BookingPanel teacher={teacher} />
          </div>
        </aside>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  children,
}: {
  label: string;
  value: string;
  children?: React.ReactNode;
}) {
  return (
    <div>
      <p className="text-sm font-bold text-muted-foreground">{label}</p>
      <p className="font-display text-2xl">{value}</p>
      {children}
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-10">
      <h2 className="font-display text-2xl">{title}</h2>
      <div className="mt-3">{children}</div>
    </section>
  );
}

/** Renders \n\n-separated plain text as paragraphs at a readable measure. */
function Prose({ text }: { text: string }) {
  return (
    <div className="max-w-[70ch] space-y-4 text-base leading-relaxed text-foreground/90">
      {text.split("\n\n").map((para, i) => (
        <p key={i}>{para}</p>
      ))}
    </div>
  );
}
