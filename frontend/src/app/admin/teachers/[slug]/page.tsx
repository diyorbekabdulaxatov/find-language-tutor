import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { TeacherModeration } from "@/features/admin/components/teacher-moderation";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaTeacher") };
}

export default async function AdminTeacherPage({
  params,
}: PageProps<"/admin/teachers/[slug]">) {
  const { slug } = await params;
  return <TeacherModeration slug={slug} />;
}
