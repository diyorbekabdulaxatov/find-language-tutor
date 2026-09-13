import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { CoursePlayer } from "@/features/courses/components/course-player";

export const metadata: Metadata = { title: "Learning" };

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
