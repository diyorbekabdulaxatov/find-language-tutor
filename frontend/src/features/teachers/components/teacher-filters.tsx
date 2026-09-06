"use client";

import { useCallback, useTransition } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import type { TeacherListResult, TeacherSort } from "@/features/teachers/api";
import { Label } from "@/components/ui/label";
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
  { value: "professional", label: "Professional teachers" },
  { value: "community", label: "Community tutors" },
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
  const hasFilters = Boolean(lang || kind || params.get("max"));

  return (
    <div className={cn("space-y-7", isPending && "opacity-60 transition-opacity")}>
      <div className="flex items-center justify-between">
        <h2 className="font-display text-base font-medium">Filters</h2>
        {hasFilters && (
          <button
            type="button"
            onClick={() => commit((next) => {
              next.delete("lang");
              next.delete("kind");
              next.delete("max");
            })}
            className="text-sm text-[var(--color-link)] hover:underline"
          >
            Clear all
          </button>
        )}
      </div>

      <Field label="Language">
        <Select value={lang || "any"} onValueChange={(v) => setParam("lang", v === "any" ? "" : v)}>
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="any">Any language</SelectItem>
            {facets.languages.map((l) => (
              <SelectItem key={l.code} value={l.code}>
                {l.name} ({l.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      <Field label="Teacher type">
        <div className="space-y-1.5">
          {KIND_OPTIONS.map((opt) => (
            <label
              key={opt.value || "any"}
              className="flex cursor-pointer items-center gap-2.5 text-sm"
            >
              <input
                type="radio"
                name="kind"
                checked={kind === opt.value}
                onChange={() => setParam("kind", opt.value)}
                className="size-3.5 accent-primary"
              />
              {opt.label}
            </label>
          ))}
        </div>
      </Field>

      <Field label={`Up to ${som.format(max)} so'm / hour`}>
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
          onValueCommit={([v]) =>
            setParam("max", v >= PRICE_MAX ? "" : String(v))
          }
          className="py-2"
        />
      </Field>

      <Field label="Sort by">
        <Select value={sort} onValueChange={(v) => setParam("sort", v)}>
          <SelectTrigger className="w-full">
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
      </Field>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-2">
      <Label className="text-sm font-medium text-foreground">{label}</Label>
      {children}
    </div>
  );
}
