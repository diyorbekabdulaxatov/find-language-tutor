"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Star } from "lucide-react";
import { cn } from "@/lib/utils";

/** Read-only star row. */
export function Stars({
  value,
  className,
}: {
  value: number;
  className?: string;
}) {
  const t = useTranslations("reviews");
  return (
    <span className={cn("inline-flex gap-0.5", className)} aria-label={t("outOf5", { value })}>
      {[1, 2, 3, 4, 5].map((n) => (
        <Star
          key={n}
          className={cn(
            "size-4",
            n <= Math.round(value)
              ? "fill-star text-star"
              : "fill-transparent text-muted-foreground/40",
          )}
          aria-hidden
        />
      ))}
    </span>
  );
}

/** Interactive 1–5 star picker. */
export function StarInput({
  value,
  onChange,
}: {
  value: number;
  onChange: (v: number) => void;
}) {
  const [hover, setHover] = useState(0);
  const shown = hover || value;
  const t = useTranslations("reviews");

  return (
    <div className="flex gap-1" onMouseLeave={() => setHover(0)}>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          aria-label={t("stars", { count: n })}
          aria-pressed={value === n}
          onMouseEnter={() => setHover(n)}
          onClick={() => onChange(n)}
          className="rounded p-0.5 outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        >
          <Star
            className={cn(
              "size-7 transition-colors",
              n <= shown
                ? "fill-star text-star"
                : "fill-transparent text-muted-foreground/40",
            )}
          />
        </button>
      ))}
    </div>
  );
}
