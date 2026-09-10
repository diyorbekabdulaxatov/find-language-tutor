import type { Metadata } from "next";
import { TeacherModeration } from "@/features/admin/components/teacher-moderation";

export const metadata: Metadata = { title: "Teacher" };

export default async function AdminTeacherPage({
  params,
}: PageProps<"/admin/teachers/[slug]">) {
  const { slug } = await params;
  return <TeacherModeration slug={slug} />;
}
