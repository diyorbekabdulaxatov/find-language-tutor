import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { PayoutBatchStatus } from "@/features/admin/api";

const STYLES: Record<PayoutBatchStatus, string> = {
  completed: "bg-primary/15 text-primary",
  processing: "bg-star/15 text-star",
  failed: "bg-destructive/10 text-destructive",
};

export function PayoutBatchStatusBadge({ status }: { status: PayoutBatchStatus }) {
  const t = useTranslations("admin.batchStatus");
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
