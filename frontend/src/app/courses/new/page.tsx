import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { CourseEditor } from "@/features/courses/components/course-editor";

export const metadata: Metadata = { title: "New course" };

export default function NewCoursePage() {
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
          <CourseEditor />
        </div>
      </RequireUser>
    </Suspense>
  );
}
