import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { CourseLibrary } from "@/features/courses/components/course-library";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaLibrary") };
}

export default function CoursesPage() {
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6">
          <CourseLibrary />
        </div>
      </RequireUser>
    </Suspense>
  );
}
