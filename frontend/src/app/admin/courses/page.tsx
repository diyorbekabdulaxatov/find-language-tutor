import type { Metadata } from "next";
import { CourseModeration } from "@/features/admin/components/course-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Courses" };

export default function AdminCoursesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.coursesModerate}>
      <CourseModeration />
    </PermissionGate>
  );
}
