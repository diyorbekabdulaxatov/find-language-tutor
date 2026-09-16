import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { BookingStatus } from "@/features/bookings/api";

const STYLES: Record<BookingStatus, string> = {
  pending_payment: "bg-star/15 text-star",
  confirmed: "bg-primary/15 text-primary",
  completed: "bg-muted text-muted-foreground",
  cancelled: "bg-destructive/10 text-destructive",
};

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  const t = useTranslations("bookingStatus");
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold",
        STYLES[status],
      )}
    >
      {t(status)}
    </span>
  );
}
