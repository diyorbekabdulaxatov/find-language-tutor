import { MetricsDashboard } from "@/features/admin/components/metrics-dashboard";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export default function AdminDashboardPage() {
  return (
    <PermissionGate permission={PERMISSIONS.metricsView}>
      <MetricsDashboard />
    </PermissionGate>
  );
}
