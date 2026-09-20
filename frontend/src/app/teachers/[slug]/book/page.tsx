import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { ArrowLeft } from "lucide-react";
import { getLessonTypes, getTeacherBySlug } from "@/features/teachers/api";
import { BookingFlow } from "@/features/bookings/components/booking-flow";

export const dynamicParams = true;

export async function generateMetadata({
  params,
}: PageProps<"/teachers/[slug]/book">): Promise<Metadata> {
  const { slug } = await params;
  const [t, teacher] = await Promise.all([getTranslations("bookPage"), getTeacherBySlug(slug)]);
  return teacher ? { title: t("metaTitle", { name: teacher.displayName }) } : {};
}

export default async function BookPage({
  params,
  searchParams,
}: PageProps<"/teachers/[slug]/book">) {
  const { slug } = await params;
  const sp = await searchParams;
  const [t, teacher, lessonTypes] = await Promise.all([
    getTranslations("bookPage"),
    getTeacherBySlug(slug),
    getLessonTypes(slug),
  ]);
  if (!teacher) notFound();

  // The profile links straight to an offering (?lesson=&duration=); ?trial=1 is
  // the pre-lesson-type link and maps onto the teacher's trial offering.
  const requested = typeof sp.lesson === "string" ? sp.lesson : undefined;
  const trialWanted = sp.trial === "1";
  const selected =
    lessonTypes.find((lt) => lt.id === requested) ??
    (trialWanted ? lessonTypes.find((lt) => lt.isTrial) : undefined);
  const requestedDuration = Number(
    typeof sp.duration === "string" ? sp.duration : Number.NaN,
  );

  const isTrial = selected ? selected.isTrial : trialWanted && teacher.trialPrice != null;

  return (
    <div className="mx-auto max-w-[1340px] px-4 py-8 sm:px-6">
      <Link
        href={`/teachers/${slug}`}
        className="inline-flex items-center gap-1.5 text-sm font-bold text-link hover:underline"
      >
        <ArrowLeft className="size-4" />
        {t("backTo", { name: teacher.displayName })}
      </Link>

      <h1 className="mt-3 font-display text-[1.75rem] sm:text-[2rem]">
        {t(isTrial ? "bookTrial" : "bookLesson")}
      </h1>

      <div className="mt-6">
        <BookingFlow
          slug={slug}
          teacherName={teacher.displayName}
          teacherAvatarUrl={teacher.avatarUrl}
          teacherTimezone={teacher.timezone}
          listPrice={
            selected
              ? selected.from
              : isTrial && teacher.trialPrice
                ? teacher.trialPrice
                : teacher.pricePerHour
          }
          isTrial={isTrial}
          lessonTypes={lessonTypes}
          selectedLessonTypeId={selected?.id}
          selectedDuration={
            Number.isFinite(requestedDuration) ? requestedDuration : undefined
          }
        />
      </div>
    </div>
  );
}
