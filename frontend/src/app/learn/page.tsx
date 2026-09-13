import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { MyLearning } from "@/features/courses/components/my-learning";

export const metadata: Metadata = { title: "My learning" };

export default function LearnPage() {
  return (
    <Suspense>
      <RequireUser>
        <MyLearning />
      </RequireUser>
    </Suspense>
  );
}
