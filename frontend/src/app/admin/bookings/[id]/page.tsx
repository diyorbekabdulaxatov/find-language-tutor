import type { Metadata } from "next";
import { BookingModeration } from "@/features/admin/components/booking-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export const metadata: Metadata = { title: "Booking" };

export default async function AdminBookingPage({
  params,
}: PageProps<"/admin/bookings/[id]">) {
  const { id } = await params;
  return (
    <PermissionGate permission={PERMISSIONS.bookingsView}>
      <BookingModeration id={id} />
    </PermissionGate>
  );
}
