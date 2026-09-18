import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { CourseReviewsModeration } from "@/features/admin/components/course-reviews-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaCourseReviews") };
}

export default function AdminCourseReviewsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.reviewsModerate}>
      <CourseReviewsModeration />
    </PermissionGate>
  );
}
