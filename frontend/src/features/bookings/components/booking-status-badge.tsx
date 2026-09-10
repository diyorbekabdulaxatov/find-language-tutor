import { cn } from "@/lib/utils";
import type { BookingStatus } from "@/features/bookings/api";

const STYLES: Record<BookingStatus, { label: string; className: string }> = {
  pending_payment: {
    label: "Pending payment",
    className: "bg-star/15 text-star",
  },
  confirmed: { label: "Confirmed", className: "bg-primary/15 text-primary" },
  completed: { label: "Completed", className: "bg-muted text-muted-foreground" },
  cancelled: {
    label: "Cancelled",
    className: "bg-destructive/10 text-destructive",
  },
};

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  const s = STYLES[status];
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold",
        s.className,
      )}
    >
      {s.label}
    </span>
  );
}
