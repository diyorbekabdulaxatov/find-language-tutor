"use client";

import { useCallback, useState, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { ChevronDown, Search, SlidersHorizontal } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { CourseSort } from "@/features/courses/types";

const SORT_OPTIONS: {
  value: CourseSort;
  label: "sortNewest" | "sortPriceAsc" | "sortPriceDesc" | "sortRating";
}[] = [
  { value: "rating", label: "sortRating" },
  { value: "newest", label: "sortNewest" },
  { value: "price_asc", label: "sortPriceAsc" },
  { value: "price_desc", label: "sortPriceDesc" },
];

/** Price ceilings in UZS minor units (tiyin). "" = no ceiling. */
const PRICE_OPTIONS: { value: string; label: "priceAll" | "priceFree" | "priceUnder100" | "priceUnder300" }[] = [
  { value: "", label: "priceAll" },
  { value: "0", label: "priceFree" },
  { value: "10000000", label: "priceUnder100" },
  { value: "30000000", label: "priceUnder300" },
];

function useCatalogParams() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [isPending, startTransition] = useTransition();

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
  return { params, setParam, isPending };
}

/**
 * Udemy's results toolbar: a "Filter" toggle (phones — the sidebar is always
 * open on desktop), the sort select, the search field, and the result count
 * on the right.
 */
export function CourseCatalogToolbar({
  total,
  onToggleFilters,
}: {
  total: number;
  onToggleFilters?: () => void;
}) {
  const { params, setParam, isPending } = useCatalogParams();
  const [q, setQ] = useState(params.get("q") ?? "");
  const t = useTranslations("courses");
  const sort = (params.get("sort") as CourseSort) ?? "rating";

  return (
    <div
      className={cn(
        "flex flex-col gap-3 transition-opacity sm:flex-row sm:items-center",
        isPending && "opacity-60",
      )}
    >
      {onToggleFilters && (
        <button
          type="button"
          onClick={onToggleFilters}
          className="inline-flex h-12 items-center gap-2 rounded-md border border-foreground bg-background px-4 text-sm font-bold text-foreground hover:bg-accent lg:hidden"
        >
          <SlidersHorizontal className="size-4" />
          {t("filter")}
        </button>
      )}

      <Select value={sort} onValueChange={(v) => setParam("sort", v === "rating" ? "" : v)}>
        <SelectTrigger className="h-12 border-foreground font-bold sm:w-[220px]">
          <span className="flex flex-col items-start leading-none">
            <span className="text-[0.65rem] font-normal text-muted-foreground">{t("sortBy")}</span>
            <SelectValue>
              {t(SORT_OPTIONS.find((opt) => opt.value === sort)?.label ?? "sortRating")}
            </SelectValue>
          </span>
        </SelectTrigger>
        <SelectContent>
          {SORT_OPTIONS.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {t(opt.label)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <form
        className="relative flex-1"
        onSubmit={(e) => {
          e.preventDefault();
          setParam("q", q.trim());
        }}
      >
        <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onBlur={() => setParam("q", q.trim())}
          type="search"
          placeholder={t("searchPlaceholder")}
          className="h-12 w-full rounded-md border border-foreground bg-background pr-3 pl-9 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/30"
        />
      </form>

      <p className="text-sm font-bold text-muted-foreground sm:ml-auto">
        {t("results", { count: total })}
      </p>
    </div>
  );
}

/** Udemy's left sidebar: accordion groups of radio filters. */
export function CourseCatalogSidebar({ className }: { className?: string }) {
  const { params, setParam } = useCatalogParams();
  const t = useTranslations("courses");
  const price = params.get("max_price") ?? "";

  return (
    <aside className={cn("text-sm", className)}>
      <FilterGroup title={t("priceFilter")}>
        {PRICE_OPTIONS.map((opt) => (
          <label key={opt.value} className="flex cursor-pointer items-center gap-3 py-1.5">
            <input
              type="radio"
              name="price"
              value={opt.value}
              checked={price === opt.value}
              onChange={() => setParam("max_price", opt.value)}
              className="size-4 accent-ink"
            />
            <span>{t(opt.label)}</span>
          </label>
        ))}
      </FilterGroup>
    </aside>
  );
}

function FilterGroup({ title, children }: { title: string; children: React.ReactNode }) {
  const [open, setOpen] = useState(true);
  return (
    <div className="border-t border-border py-3">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center justify-between py-1 text-base font-bold text-foreground"
      >
        {title}
        <ChevronDown className={cn("size-4 transition-transform", open && "rotate-180")} />
      </button>
      {open && <div className="mt-1 flex flex-col">{children}</div>}
    </div>
  );
}

/** Wraps sidebar + results so the phone "Filter" toggle can show/hide the sidebar. */
export function CourseCatalogLayout({
  total,
  children,
}: {
  total: number;
  children: React.ReactNode;
}) {
  const [filtersOpen, setFiltersOpen] = useState(false);
  return (
    <div>
      <CourseCatalogToolbar total={total} onToggleFilters={() => setFiltersOpen((o) => !o)} />
      <div className="mt-6 grid gap-8 lg:grid-cols-[260px_1fr]">
        <CourseCatalogSidebar className={cn(filtersOpen ? "block" : "hidden lg:block")} />
        <div>{children}</div>
      </div>
    </div>
  );
}
