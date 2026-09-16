import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { RolesManager } from "@/features/admin/components/roles-manager";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaRoles") };
}

export default function AdminRolesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.rolesManage}>
      <RolesManager />
    </PermissionGate>
  );
}
