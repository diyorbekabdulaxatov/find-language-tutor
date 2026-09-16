import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { ResourceLibrary } from "@/features/resources/components/resource-library";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("resources");
  return { title: t("meta") };
}

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
