import type { Metadata } from "next";
import { Suspense } from "react";
import { TeachersTable } from "@/features/admin/components/teachers-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Teachers" };

export default function AdminTeachersPage() {
  return (
    <PermissionGate permission={PERMISSIONS.teachersView}>
      <Suspense>
        <TeachersTable />
      </Suspense>
    </PermissionGate>
  );
}
