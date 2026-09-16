import { useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import type { TeacherStatus } from "@/features/admin/api";

const STYLES: Record<TeacherStatus, string> = {
  pending: "bg-star/15 text-star",
  approved: "bg-primary/15 text-primary",
  rejected: "bg-destructive/10 text-destructive",
  suspended: "bg-destructive/10 text-destructive",
};

export function TeacherStatusBadge({ status }: { status: TeacherStatus }) {
  const t = useTranslations("admin.teacherStatus");
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
