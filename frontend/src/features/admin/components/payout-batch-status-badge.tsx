import { cn } from "@/lib/utils";
import type { PayoutBatchStatus } from "@/features/admin/api";

const STYLES: Record<PayoutBatchStatus, { label: string; className: string }> = {
  completed: { label: "Completed", className: "bg-primary/15 text-primary" },
  processing: { label: "Processing", className: "bg-star/15 text-star" },
  failed: { label: "Failed", className: "bg-destructive/10 text-destructive" },
};

export function PayoutBatchStatusBadge({
  status,
}: {
  status: PayoutBatchStatus;
}) {
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
