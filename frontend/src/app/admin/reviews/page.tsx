import type { Metadata } from "next";
import { ReviewsModeration } from "@/features/admin/components/reviews-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Reviews" };

export default function AdminReviewsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.reviewsModerate}>
      <ReviewsModeration />
    </PermissionGate>
  );
}
