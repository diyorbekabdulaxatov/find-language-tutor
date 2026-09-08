import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { getTeacherBySlug } from "@/features/teachers/api";
import { BookingFlow } from "@/features/bookings/components/booking-flow";

export const dynamicParams = true;

export async function generateMetadata({
  params,
}: PageProps<"/teachers/[slug]/book">): Promise<Metadata> {
  const { slug } = await params;
  const teacher = await getTeacherBySlug(slug);
  return teacher
    ? { title: `Book a lesson with ${teacher.displayName}` }
    : {};
}

export default async function BookPage({
  params,
  searchParams,
}: PageProps<"/teachers/[slug]/book">) {
  const { slug } = await params;
  const sp = await searchParams;
  const teacher = await getTeacherBySlug(slug);
  if (!teacher) notFound();

  const isTrial = sp.trial === "1" && teacher.trialPrice != null;

  return (
    <div className="mx-auto max-w-2xl px-4 py-8 sm:px-6">
      <Link
        href={`/teachers/${slug}`}
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        Back to {teacher.displayName}
      </Link>

      <h1 className="mt-4 font-display text-3xl">
        {isTrial ? "Book a trial lesson" : "Book a lesson"}
      </h1>

      <div className="mt-6">
        <BookingFlow
          slug={slug}
          teacherName={teacher.displayName}
          teacherTimezone={teacher.timezone}
          isTrial={isTrial}
        />
      </div>
    </div>
  );
}
