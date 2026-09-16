import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { EditCourse } from "@/features/courses/components/edit-course";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaEdit") };
}

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
