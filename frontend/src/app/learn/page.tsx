import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { MyLearning } from "@/features/courses/components/my-learning";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("courses");
  return { title: t("metaLearn") };
}

export default function LearnPage() {
  return (
    <Suspense>
      <RequireUser>
        <MyLearning />
      </RequireUser>
    </Suspense>
  );
}
