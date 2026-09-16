import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { BookingsList } from "@/features/bookings/components/bookings-list";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("bookings");
  return { title: t("metaList") };
}

export default function BookingsPage() {
  return (
    <Suspense>
      <RequireUser>
        <BookingsList />
      </RequireUser>
    </Suspense>
  );
}
