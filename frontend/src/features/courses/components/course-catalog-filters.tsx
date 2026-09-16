"use client";

import { useCallback, useState, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CourseSort } from "@/features/courses/types";

const SORT_OPTIONS: { value: CourseSort; label: "sortNewest" | "sortPriceAsc" | "sortPriceDesc" }[] = [
  { value: "newest", label: "sortNewest" },
  { value: "price_asc", label: "sortPriceAsc" },
  { value: "price_desc", label: "sortPriceDesc" },
];

/**
 * Search + sort bar for the public course catalog. Mirrors `TeacherFilters`'
 * URL-driven-state approach, scaled down — no facets endpoint exists for
 * courses, so this is just `q` and `sort`.
 */
export function CourseCatalogFilters() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [isPending, startTransition] = useTransition();
  const [q, setQ] = useState(params.get("q") ?? "");
  const t = useTranslations("courses");

  const setParam = useCallback(
    (key: string, value: string) => {
      const next = new URLSearchParams(params.toString());
      if (value) next.set(key, value);
      else next.delete(key);
      startTransition(() => {
        router.replace(next.toString() ? `${pathname}?${next}` : pathname, {
          scroll: false,
        });
      });
    },
    [params, pathname, router],
  );

  const sort = (params.get("sort") as CourseSort) ?? "newest";

  return (
    <div
      className={cn(
        "flex flex-col gap-3 rounded-2xl bg-card p-4 ring-1 ring-border shadow-soft transition-opacity sm:flex-row sm:items-center sm:p-5",
        isPending && "opacity-60",
      )}
    >
      <form
        className="relative flex-1"
        onSubmit={(e) => {
          e.preventDefault();
          setParam("q", q.trim());
        }}
      >
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onBlur={() => setParam("q", q.trim())}
          type="search"
          placeholder={t("searchPlaceholder")}
          className="h-10 w-full rounded-lg border border-input bg-transparent pl-9 pr-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </form>

      <Select value={sort} onValueChange={(v) => setParam("sort", v === "newest" ? "" : v)}>
        <SelectTrigger className="h-10 sm:w-[190px]">
          <SelectValue>{t(SORT_OPTIONS.find((opt) => opt.value === sort)?.label ?? "sortNewest")}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          {SORT_OPTIONS.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {t(opt.label)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
