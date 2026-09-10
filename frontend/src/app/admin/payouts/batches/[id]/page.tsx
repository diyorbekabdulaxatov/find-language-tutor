import type { Metadata } from "next";
import { PayoutBatchDetail } from "@/features/admin/components/payout-batch-detail";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Payout batch" };

export default async function AdminPayoutBatchPage({
  params,
}: PageProps<"/admin/payouts/batches/[id]">) {
  const { id } = await params;
  return (
    <PermissionGate permission={PERMISSIONS.payoutsView}>
      <PayoutBatchDetail id={id} />
    </PermissionGate>
  );
}
