import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { ReviewsModeration } from "@/features/admin/components/reviews-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaReviews") };
}

export default function AdminReviewsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.reviewsModerate}>
      <ReviewsModeration />
    </PermissionGate>
  );
}
