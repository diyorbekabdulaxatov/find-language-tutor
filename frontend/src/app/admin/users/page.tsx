import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { UsersTable } from "@/features/admin/components/users-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaUsers") };
}

export default function AdminUsersPage() {
  return (
    <PermissionGate permission={PERMISSIONS.usersView}>
      <UsersTable />
    </PermissionGate>
  );
}
