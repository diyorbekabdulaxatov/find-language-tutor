import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { CourseModeration } from "@/features/admin/components/course-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaCourses") };
}

export default function AdminCoursesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.coursesModerate}>
      <CourseModeration />
    </PermissionGate>
  );
}
