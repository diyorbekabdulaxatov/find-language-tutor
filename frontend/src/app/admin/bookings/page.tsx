import type { Metadata } from "next";
import { Suspense } from "react";
import { BookingsTable } from "@/features/admin/components/bookings-table";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Bookings" };

export default function AdminBookingsPage() {
  return (
    <PermissionGate permission={PERMISSIONS.bookingsView}>
      <Suspense>
        <BookingsTable />
      </Suspense>
    </PermissionGate>
  );
}
