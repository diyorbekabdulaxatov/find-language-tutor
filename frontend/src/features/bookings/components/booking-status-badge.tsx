import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { BookingStatus } from "@/features/bookings/api";

const STYLES: Record<BookingStatus, string> = {
  pending_payment: "bg-star/20 text-rating dark:text-star",
  confirmed: "bg-accent text-accent-foreground",
  completed: "bg-muted text-muted-foreground",
  cancelled: "bg-destructive/10 text-destructive",
};

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  const t = useTranslations("bookingStatus");
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-sm px-2 py-0.5 text-xs font-bold",
        STYLES[status],
      )}
    >
      {t(status)}
    </span>
  );
}
