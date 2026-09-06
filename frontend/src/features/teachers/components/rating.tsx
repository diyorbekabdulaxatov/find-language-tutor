import { Star } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatCompact } from "@/lib/format";

export function Rating({
  value,
  reviewCount,
  className,
}: {
  value: number;
  reviewCount?: number;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-baseline gap-1.5", className)}>
      <Star className="size-3.5 translate-y-0.5 fill-primary text-primary" aria-hidden />
      <span className="font-medium tabular-nums">{value.toFixed(1)}</span>
      {reviewCount != null && (
        <span className="text-muted-foreground">
          ({formatCompact(reviewCount)} review{reviewCount === 1 ? "" : "s"})
        </span>
      )}
    </span>
  );
}
