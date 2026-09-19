import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireAdmin } from "@/features/admin/require-admin";
import { AdminSidebar, AdminTabs } from "@/features/admin/admin-nav";

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
        <div className="lg:grid lg:min-h-[calc(100vh-72px)] lg:grid-cols-[240px_minmax(0,1fr)] lg:items-start">
          <AdminSidebar />

          <div className="mx-auto w-full max-w-[1340px] px-4 py-8 sm:px-6">
            <h1 className="font-display text-3xl sm:text-[2rem]">{t("title")}</h1>
            <div className="mt-4">
              <AdminTabs />
            </div>
            <div className="mt-6">{children}</div>
          </div>
        </div>
      </RequireAdmin>
    </Suspense>
  );
}
