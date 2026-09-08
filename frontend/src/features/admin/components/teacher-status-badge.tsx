import { cn } from "@/lib/utils";
import type { TeacherStatus } from "@/features/admin/api";

const STYLES: Record<TeacherStatus, { label: string; className: string }> = {
  pending: { label: "Pending", className: "bg-star/15 text-star" },
  approved: { label: "Approved", className: "bg-primary/15 text-primary" },
  rejected: { label: "Rejected", className: "bg-destructive/10 text-destructive" },
  suspended: {
    label: "Suspended",
    className: "bg-destructive/10 text-destructive",
  },
};

export function TeacherStatusBadge({ status }: { status: TeacherStatus }) {
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
