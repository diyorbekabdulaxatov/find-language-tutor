import type { Metadata } from "next";
import { RolesManager } from "@/features/admin/components/roles-manager";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Roles" };

export default function AdminRolesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.rolesManage}>
      <RolesManager />
    </PermissionGate>
  );
}
