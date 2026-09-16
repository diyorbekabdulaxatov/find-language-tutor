import { useTranslations } from "next-intl";
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
  const t = useTranslations("profile");
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-primary",
        withLabel && "text-xs font-medium",
        className,
      )}
      title={t("verifiedTitle")}
    >
      <BadgeCheck className={cn(withLabel ? "size-3.5" : "size-4")} aria-hidden />
      {withLabel && t("verified")}
      {!withLabel && <span className="sr-only">{t("verified")}</span>}
    </span>
  );
}
