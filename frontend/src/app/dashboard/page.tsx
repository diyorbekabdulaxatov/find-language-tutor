import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { DashboardShell } from "@/features/dashboard/components/dashboard-shell";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("dashboard");
  return { title: t("meta") };
}

export default function DashboardPage() {
  return (
    <Suspense>
      <RequireUser>
        <DashboardShell />
      </RequireUser>
    </Suspense>
  );
}
