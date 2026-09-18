import { useLocale, useTranslations } from "next-intl";
import { cn } from "@/lib/utils";
import { formatCompact } from "@/lib/format";
import { Stars } from "@/features/reviews/components/star-rating";

/**
 * Udemy's rating row: the number in bold brown, amber stars, the count in
 * grey parentheses. `pill` is the compact card form (compact count),
 * `inline` the masthead form (spelled-out "N reviews").
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
  const t = useTranslations("profile");
  const locale = useLocale();

  if (variant === "pill") {
    return (
      <span className={cn("inline-flex items-center gap-1 text-xs", className)}>
        <span className="font-bold tabular-nums text-rating">{value.toFixed(1)}</span>
        <Stars value={value} className="[&_svg]:size-3.5" />
        {reviewCount != null && (
          <span className="text-muted-foreground">({formatCompact(reviewCount, locale)})</span>
        )}
      </span>
    );
  }

  return (
    <span className={cn("inline-flex items-center gap-1.5 text-sm", className)}>
      <span className="font-bold tabular-nums text-rating">{value.toFixed(1)}</span>
      <Stars value={value} />
      {reviewCount != null && (
        <span className="text-muted-foreground">
          ({t("reviewsCount", { count: reviewCount })})
        </span>
      )}
    </span>
  );
}
