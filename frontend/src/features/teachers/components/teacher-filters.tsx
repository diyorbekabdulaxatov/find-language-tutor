"use client";

import { useCallback, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { SlidersHorizontal, X } from "lucide-react";
import type { TeacherListResult, TeacherSort } from "@/features/teachers/api";
import { Slider } from "@/components/ui/slider";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

const SORT_OPTIONS: { value: TeacherSort; label: string }[] = [
  { value: "recommended", label: "Recommended" },
  { value: "price_asc", label: "Price: low to high" },
  { value: "price_desc", label: "Price: high to low" },
  { value: "rating_desc", label: "Highest rated" },
];

const KIND_OPTIONS = [
  { value: "", label: "Anyone" },
  { value: "professional", label: "Professional" },
  { value: "community", label: "Community" },
];

/** Price filter works in whole so'm; kept in the URL as e.g. ?max=90000. */
const PRICE_MIN = 30_000;
const PRICE_MAX = 150_000;
const PRICE_STEP = 5_000;

const som = new Intl.NumberFormat("en-US");

export function TeacherFilters({
  facets,
}: {
  facets: TeacherListResult["facets"];
}) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [isPending, startTransition] = useTransition();

  const commit = useCallback(
    (mutate: (next: URLSearchParams) => void) => {
      const next = new URLSearchParams(params.toString());
      mutate(next);
      startTransition(() => {
        router.replace(next.toString() ? `${pathname}?${next}` : pathname, {
          scroll: false,
        });
      });
    },
    [params, pathname, router],
  );

  const setParam = useCallback(
    (key: string, value: string) => {
      commit((next) => {
        if (value) next.set(key, value);
        else next.delete(key);
      });
    },
    [commit],
  );

  const lang = params.get("lang") ?? "";
  const kind = params.get("kind") ?? "";
  const sort = (params.get("sort") as TeacherSort) ?? "recommended";
  const max = Number(params.get("max") ?? PRICE_MAX);
  const priceActive = Boolean(params.get("max"));
  const hasFilters = Boolean(lang || kind || priceActive);

  return (
    <div
      className={cn(
        "rounded-2xl bg-card p-4 ring-1 ring-border shadow-soft transition-opacity sm:p-5",
        isPending && "opacity-60",
      )}
    >
      {/* Language chips */}
      <div className="flex flex-wrap gap-2">
        <Chip active={!lang} onClick={() => setParam("lang", "")}>
          All languages
        </Chip>
        {facets.languages.map((l) => (
          <Chip
            key={l.code}
            active={lang === l.code}
            onClick={() => setParam("lang", lang === l.code ? "" : l.code)}
          >
            {l.name}
            <span className={cn("ml-1", lang === l.code ? "text-primary-foreground/70" : "text-muted-foreground")}>
              {l.count}
            </span>
          </Chip>
        ))}
      </div>

      <div className="mt-4 flex flex-col gap-4 border-t border-border pt-4 lg:flex-row lg:items-center lg:gap-6">
        {/* Kind segmented control */}
        <div className="inline-flex rounded-lg bg-secondary p-0.5">
          {KIND_OPTIONS.map((opt) => (
            <button
              key={opt.value || "any"}
              type="button"
              onClick={() => setParam("kind", opt.value)}
              className={cn(
                "rounded-[calc(var(--radius)-6px)] px-3 py-1.5 text-sm font-medium transition-colors",
                kind === opt.value
                  ? "bg-card text-foreground shadow-soft"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {opt.label}
            </button>
          ))}
        </div>

        {/* Price */}
        <div className="flex min-w-[200px] flex-1 items-center gap-3">
          <SlidersHorizontal className="size-4 shrink-0 text-muted-foreground" />
          <div className="flex-1">
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>Max price</span>
              <span className="font-medium text-foreground">
                {priceActive ? `${som.format(max)} so'm` : "Any"}
              </span>
            </div>
            <Slider
              min={PRICE_MIN}
              max={PRICE_MAX}
              step={PRICE_STEP}
              value={[Math.min(Math.max(max, PRICE_MIN), PRICE_MAX)]}
              onValueChange={([v]) => {
                const next = new URLSearchParams(params.toString());
                next.set("max", String(v));
                router.replace(`${pathname}?${next}`, { scroll: false });
              }}
              onValueCommit={([v]) => setParam("max", v >= PRICE_MAX ? "" : String(v))}
              className="mt-1.5"
            />
          </div>
        </div>

        {/* Sort */}
        <Select value={sort} onValueChange={(v) => setParam("sort", v)}>
          <SelectTrigger className="h-9 lg:w-[190px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {SORT_OPTIONS.map((opt) => (
              <SelectItem key={opt.value} value={opt.value}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {hasFilters && (
          <button
            type="button"
            onClick={() =>
              commit((next) => {
                next.delete("lang");
                next.delete("kind");
                next.delete("max");
              })
            }
            className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
          >
            <X className="size-3.5" />
            Clear
          </button>
        )}
      </div>
    </div>
  );
}

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        "rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors",
        active
          ? "bg-primary text-primary-foreground"
          : "bg-secondary text-secondary-foreground hover:bg-accent hover:text-accent-foreground",
      )}
    >
      {children}
    </button>
  );
}
