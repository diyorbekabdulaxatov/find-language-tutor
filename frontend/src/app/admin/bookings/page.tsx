import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { BookingsTable } from "@/features/admin/components/bookings-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaBookings") };
}

export default function AdminBookingsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.bookingsView}>
      <Suspense>
        <BookingsTable />
      </Suspense>
    </PermissionGate>
  );
}
