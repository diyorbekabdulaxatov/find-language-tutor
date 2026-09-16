import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { CourseEditor } from "@/features/courses/components/course-editor";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaNew") };
}

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
