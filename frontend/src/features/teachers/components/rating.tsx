import { Star } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatCompact } from "@/lib/format";

/**
 * `pill` — amber chip, for cards and dense spots.
 * default — inline star + number + review count, for the profile masthead.
 */
export function Rating({
  value,
  reviewCount,
  variant = "inline",
  className,
}: {
  value: number;
  reviewCount?: number;
  variant?: "inline" | "pill";
  className?: string;
}) {
  if (variant === "pill") {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1 rounded-full bg-star/12 px-2 py-0.5 text-xs font-semibold text-star",
          className,
        )}
      >
        <Star className="size-3 fill-current" aria-hidden />
        <span className="tabular-nums">{value.toFixed(1)}</span>
        {reviewCount != null && (
          <span className="font-medium text-star/80">
            ({formatCompact(reviewCount)})
          </span>
        )}
      </span>
    );
  }

  return (
    <span className={cn("inline-flex items-baseline gap-1.5", className)}>
      <Star className="size-4 translate-y-0.5 fill-star text-star" aria-hidden />
      <span className="font-semibold tabular-nums">{value.toFixed(1)}</span>
      {reviewCount != null && (
        <span className="text-muted-foreground">
          ({formatCompact(reviewCount)} review{reviewCount === 1 ? "" : "s"})
        </span>
      )}
    </span>
  );
}
