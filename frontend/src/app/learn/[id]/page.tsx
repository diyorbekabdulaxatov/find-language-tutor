import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { CoursePlayer } from "@/features/courses/components/course-player";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaPlayer") };
}

export default async function CoursePlayerPage({
  params,
}: PageProps<"/learn/[id]">) {
  const { id } = await params;
  return (
    <Suspense>
      <RequireUser>
        <CoursePlayer id={id} />
      </RequireUser>
    </Suspense>
  );
}
