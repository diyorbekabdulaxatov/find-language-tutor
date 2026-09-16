import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { BookingModeration } from "@/features/admin/components/booking-moderation";
import { PermissionGate } from "@/features/admin/permission-gate";
import { PERMISSIONS } from "@/features/admin/permissions";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("admin");
  return { title: t("metaBooking") };
}

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
