import type { Metadata } from "next";
import { getCourseCatalogDetail } from "@/features/courses/api";
import { CourseLanding } from "@/features/courses/components/course-landing";

export async function generateMetadata({
  params,
}: PageProps<"/courses/catalog/[id]">): Promise<Metadata> {
  const { id } = await params;
  const course = await getCourseCatalogDetail(id);
  if (!course) return {};
  return {
    title: `${course.title} — ${course.teacher.displayName}`,
    description: course.subtitle || course.description,
  };
}

/**
 * Thin server wrapper: metadata comes from the unauthenticated public read
 * (good for SEO/crawlers), but the page body is client-rendered so a
 * signed-in viewer's bearer token can personalise is_enrolled / is_owner —
 * see `CourseLanding`.
 */
export default async function CourseLandingPage({
  params,
}: PageProps<"/courses/catalog/[id]">) {
  const { id } = await params;
  return <CourseLanding id={id} />;
}
