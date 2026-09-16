import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { TeachersTable } from "@/features/admin/components/teachers-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaTeachers") };
}

export default function AdminTeachersPage() {
  return (
    <PermissionGate permission={PERMISSIONS.teachersView}>
      <Suspense>
        <TeachersTable />
      </Suspense>
    </PermissionGate>
  );
}
