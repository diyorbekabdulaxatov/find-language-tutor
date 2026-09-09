import type { Metadata } from "next";
import { DisputesQueue } from "@/features/admin/components/disputes-queue";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Disputes" };

export default function AdminDisputesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.disputesResolve}>
      <DisputesQueue />
    </PermissionGate>
  );
}
