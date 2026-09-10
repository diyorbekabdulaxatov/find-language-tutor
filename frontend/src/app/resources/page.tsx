import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { ResourceLibrary } from "@/features/resources/components/resource-library";

export const metadata: Metadata = { title: "Teaching resources" };

export default function ResourcesPage() {
  return (
    <Suspense>
      <RequireUser>
        <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6">
          <ResourceLibrary />
        </div>
      </RequireUser>
    </Suspense>
  );
}
