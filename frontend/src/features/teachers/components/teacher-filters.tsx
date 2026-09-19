"use client";

import { useCallback, useState, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { ChevronDown, SlidersHorizontal, X } from "lucide-react";
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
import { languageName } from "@/lib/i18n";

const SORT_OPTIONS: { value: TeacherSort; label: SortKey }[] = [
  { value: "recommended", label: "sortRecommended" },
  { value: "rating_desc", label: "sortRatingDesc" },
  { value: "price_asc", label: "sortPriceAsc" },
  { value: "price_desc", label: "sortPriceDesc" },
];
type SortKey = "sortRecommended" | "sortPriceAsc" | "sortPriceDesc" | "sortRatingDesc";

const KIND_OPTIONS: { value: string; label: "anyone" | "professional" | "community" }[] = [
  { value: "", label: "anyone" },
  { value: "professional", label: "professional" },
  { value: "community", label: "community" },
];

/** Price filter works in whole so'm; kept in the URL as e.g. ?max=90000. */
const PRICE_MIN = 30_000;
const PRICE_MAX = 150_000;
const PRICE_STEP = 5_000;

function useTeacherParams() {
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

  return { params, pathname, router, commit, setParam, isPending };
}

/**
 * Udemy's search-results chrome: a toolbar (Filter toggle on phones, sort,
 * result count) above a two-column layout with an accordion sidebar of
 * radio filters on the left and the result list on the right.
 */
export function TeacherFilters({
  facets,
  total,
  children,
}: {
  facets: TeacherListResult["facets"];
  total: number;
  children: React.ReactNode;
}) {
  const { params, pathname, router, commit, setParam, isPending } = useTeacherParams();
  const [open, setOpen] = useState(false);
  const t = useTranslations("filters");
  const tLang = useTranslations("languages");
  const tCommon = useTranslations("common");
  const locale = useLocale();
  const som = new Intl.NumberFormat(locale);

  const lang = params.get("lang") ?? "";
  const kind = params.get("kind") ?? "";
  const sort = (params.get("sort") as TeacherSort) ?? "recommended";
  const max = Number(params.get("max") ?? PRICE_MAX);
  const priceActive = Boolean(params.get("max"));
  const hasFilters = Boolean(lang || kind || priceActive);

  return (
    <div className={cn("transition-opacity", isPending && "opacity-60")}>
      <div className="flex flex-wrap items-center gap-3">
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="inline-flex h-12 items-center gap-2 rounded-md border border-foreground bg-background px-4 text-sm font-bold text-foreground hover:bg-accent lg:hidden"
        >
          <SlidersHorizontal className="size-4" />
          {t("filter")}
        </button>

        <Select value={sort} onValueChange={(v) => setParam("sort", v === "recommended" ? "" : v)}>
          <SelectTrigger className="h-12 border-foreground font-bold sm:w-[220px]">
            <span className="flex flex-col items-start leading-none">
              <span className="text-[0.65rem] font-normal text-muted-foreground">{t("sortBy")}</span>
              <SelectValue>
                {t(SORT_OPTIONS.find((opt) => opt.value === sort)?.label ?? "sortRecommended")}
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
            className="inline-flex items-center gap-1 text-sm font-bold text-link hover:underline"
          >
            <X className="size-3.5" />
            {t("clear")}
          </button>
        )}

        <p className="ml-auto text-sm font-bold text-muted-foreground">
          {t("results", { count: total })}
        </p>
      </div>

      <div className="mt-6 grid gap-8 lg:grid-cols-[260px_1fr]">
        <aside className={cn("text-sm", open ? "block" : "hidden lg:block")}>
          <FilterGroup title={t("language")}>
            <RadioRow
              name="lang"
              checked={!lang}
              onChange={() => setParam("lang", "")}
              label={t("allLanguages")}
            />
            {facets.languages.map((l) => (
              <RadioRow
                key={l.code}
                name="lang"
                checked={lang === l.code}
                onChange={() => setParam("lang", l.code)}
                label={languageName(tLang, l)}
                count={l.count}
              />
            ))}
          </FilterGroup>

          <FilterGroup title={t("teacherType")}>
            {KIND_OPTIONS.map((opt) => (
              <RadioRow
                key={opt.value || "any"}
                name="kind"
                checked={kind === opt.value}
                onChange={() => setParam("kind", opt.value)}
                label={t(opt.label)}
              />
            ))}
          </FilterGroup>

          <FilterGroup title={t("maxPrice")}>
            <div className="flex justify-between py-1 text-xs text-muted-foreground">
              <span>{som.format(PRICE_MIN)}</span>
              <span className="font-bold text-foreground">
                {priceActive ? `${som.format(max)} ${tCommon("som")}` : t("any")}
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
          </FilterGroup>
        </aside>

        <div>{children}</div>
      </div>
    </div>
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

function RadioRow({
  name,
  checked,
  onChange,
  label,
  count,
}: {
  name: string;
  checked: boolean;
  onChange: () => void;
  label: string;
  count?: number;
}) {
  return (
    <label className="flex cursor-pointer items-center gap-3 py-1.5">
      <input
        type="radio"
        name={name}
        checked={checked}
        onChange={onChange}
        className="size-4 accent-ink"
      />
      <span className="flex-1">{label}</span>
      {count != null && <span className="text-xs text-muted-foreground">({count})</span>}
    </label>
  );
}
