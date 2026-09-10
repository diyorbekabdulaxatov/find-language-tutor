import { BadgeCheck } from "lucide-react";
import { cn } from "@/lib/utils";

/** Platform-verified teacher mark. */
export function VerifiedBadge({
  className,
  withLabel = false,
}: {
  className?: string;
  withLabel?: boolean;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-primary",
        withLabel && "text-xs font-medium",
        className,
      )}
      title="Verified by FindTutor"
    >
      <BadgeCheck className={cn(withLabel ? "size-3.5" : "size-4")} aria-hidden />
      {withLabel && "Verified"}
      {!withLabel && <span className="sr-only">Verified</span>}
    </span>
  );
}
