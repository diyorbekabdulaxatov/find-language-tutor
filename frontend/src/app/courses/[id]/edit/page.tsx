import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { EditCourse } from "@/features/courses/components/edit-course";

export const metadata: Metadata = { title: "Edit course" };

export default async function EditCoursePage({
  params,
}: PageProps<"/courses/[id]/edit">) {
  const { id } = await params;
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
          <EditCourse id={id} />
        </div>
      </RequireUser>
    </Suspense>
  );
}
