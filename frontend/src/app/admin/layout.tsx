import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireAdmin } from "@/features/admin/require-admin";
import { AdminNav } from "@/features/admin/admin-nav";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: { default: t("meta"), template: `%s · ${t("meta")} · FindTutor` } };
}

export default async function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const t = await getTranslations("admin");
  return (
    <Suspense>
      <RequireAdmin>
        <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6">
          <h1 className="font-display text-3xl">{t("title")}</h1>
          <div className="mt-4">
            <AdminNav />
          </div>
          <div className="mt-6">{children}</div>
        </div>
      </RequireAdmin>
    </Suspense>
  );
}
