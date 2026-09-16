import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Suspense } from "react";
import { RequireUser } from "@/features/auth/require-user";
import { BookingDetail } from "@/features/bookings/components/booking-detail";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("bookings");
  return { title: t("metaDetail") };
}

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
