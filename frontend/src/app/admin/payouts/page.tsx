import type { Metadata } from "next";
import { PayoutsDashboard } from "@/features/admin/components/payouts-dashboard";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Payouts" };

export default function AdminPayoutsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.payoutsView}>
      <PayoutsDashboard />
    </PermissionGate>
  );
}
