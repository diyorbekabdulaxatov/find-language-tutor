import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { PayoutsDashboard } from "@/features/admin/components/payouts-dashboard";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaPayouts") };
}

export default function AdminPayoutsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.payoutsView}>
      <PayoutsDashboard />
    </PermissionGate>
  );
}
