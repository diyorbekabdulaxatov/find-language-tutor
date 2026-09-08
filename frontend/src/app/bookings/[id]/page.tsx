import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { BookingDetail } from "@/features/bookings/components/booking-detail";

export const metadata: Metadata = { title: "Booking" };

export default async function BookingDetailPage({
  params,
}: PageProps<"/bookings/[id]">) {
  const { id } = await params;
  return (
    <Suspense>
      <RequireUser>
        <BookingDetail id={id} />
      </RequireUser>
    </Suspense>
  );
}
