import type { Metadata } from "next";
import { UsersTable } from "@/features/admin/components/users-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Users" };

export default function AdminUsersPage() {
  return (
    <PermissionGate permission={PERMISSIONS.usersView}>
      <UsersTable />
    </PermissionGate>
  );
}
