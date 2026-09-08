import type { Metadata } from "next";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { BookingsList } from "@/features/bookings/components/bookings-list";

export const metadata: Metadata = { title: "Bookings" };

export default function BookingsPage() {
  return (
    <Suspense>
      <RequireUser>
        <BookingsList />
      </RequireUser>
    </Suspense>
  );
}
