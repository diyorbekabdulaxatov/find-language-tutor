import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { TeacherStatus } from "@/features/admin/api";

const STYLES: Record<TeacherStatus, string> = {
  pending: "bg-star/20 text-rating dark:text-star",
  approved: "bg-accent text-accent-foreground",
  rejected: "bg-destructive/10 text-destructive",
  suspended: "bg-destructive/10 text-destructive",
};

export function TeacherStatusBadge({ status }: { status: TeacherStatus }) {
  const t = useTranslations("admin.teacherStatus");
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
