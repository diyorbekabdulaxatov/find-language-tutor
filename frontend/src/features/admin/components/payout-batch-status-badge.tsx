import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { PayoutBatchStatus } from "@/features/admin/api";

const STYLES: Record<PayoutBatchStatus, string> = {
  completed: "bg-accent text-accent-foreground",
  processing: "bg-star/20 text-rating dark:text-star",
  failed: "bg-destructive/10 text-destructive",
};

export function PayoutBatchStatusBadge({ status }: { status: PayoutBatchStatus }) {
  const t = useTranslations("admin.batchStatus");
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
