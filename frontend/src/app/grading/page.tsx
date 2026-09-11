import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { GradingInbox } from "@/features/submissions/components/grading-inbox";

export const metadata: Metadata = { title: "Homework to grade" };

export default function GradingPage() {
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-3xl px-4 py-10 sm:px-6">
          <GradingInbox />
        </div>
      </RequireUser>
    </Suspense>
  );
}
