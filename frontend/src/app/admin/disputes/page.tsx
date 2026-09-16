import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { DisputesQueue } from "@/features/admin/components/disputes-queue";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaDisputes") };
}

export default function AdminDisputesPage() {
  return (
    <PermissionGate permission={PERMISSIONS.disputesResolve}>
      <DisputesQueue />
    </PermissionGate>
  );
}
