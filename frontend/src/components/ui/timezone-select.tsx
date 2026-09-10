"use client";

import { useMemo, useState } from "react";
import { Check, ChevronsUpDown } from "lucide-react";
import { Popover as PopoverPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";

/** All IANA zones the runtime knows, with a sensible fallback for old engines. */
function allZones(): string[] {
  const withValues = Intl as typeof Intl & {
    supportedValuesOf?: (key: "timeZone") => string[];
  };
  if (typeof withValues.supportedValuesOf === "function") {
    return withValues.supportedValuesOf("timeZone");
  }
  return [
    "Asia/Tashkent",
    "Asia/Almaty",
    "Europe/Moscow",
    "Europe/London",
    "America/New_York",
    "America/Los_Angeles",
    "UTC",
  ];
}

/** Current UTC offset for a zone, e.g. "+05:00". */
function offsetLabel(tz: string): string {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone: tz,
      timeZoneName: "shortOffset",
    }).formatToParts(new Date());
    return parts.find((p) => p.type === "timeZoneName")?.value ?? "";
  } catch {
    return "";
  }
}

export function TimezoneSelect({
  id,
  value,
  onChange,
}: {
  id?: string;
  value: string;
  onChange: (tz: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const zones = useMemo(() => allZones(), []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q
      ? zones.filter((z) => z.toLowerCase().includes(q))
      : zones;
    return list.slice(0, 60);
  }, [zones, query]);

  return (
    <PopoverPrimitive.Root open={open} onOpenChange={setOpen}>
      <PopoverPrimitive.Trigger asChild>
        <button
          id={id}
          type="button"
          className="flex h-9 w-full items-center justify-between rounded-lg border border-input bg-transparent px-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <span className={cn(!value && "text-muted-foreground")}>
            {value || "Select a timezone"}
            {value && (
              <span className="ml-2 text-muted-foreground">
                {offsetLabel(value)}
              </span>
            )}
          </span>
          <ChevronsUpDown className="size-4 shrink-0 text-muted-foreground" />
        </button>
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          align="start"
          sideOffset={4}
          className="z-50 w-[--radix-popover-trigger-width] overflow-hidden rounded-xl border border-border bg-popover p-1 shadow-lift"
        >
          <input
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search zones…"
            className="mb-1 w-full rounded-lg bg-transparent px-2.5 py-2 text-sm outline-none"
          />
          <div className="max-h-64 overflow-y-auto">
            {filtered.length === 0 && (
              <p className="px-2.5 py-2 text-sm text-muted-foreground">
                No match.
              </p>
            )}
            {filtered.map((z) => (
              <button
                key={z}
                type="button"
                onClick={() => {
                  onChange(z);
                  setOpen(false);
                  setQuery("");
                }}
                className="flex w-full items-center justify-between gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm hover:bg-accent"
              >
                <span className="flex items-center gap-2">
                  <Check
                    className={cn(
                      "size-3.5",
                      z === value ? "opacity-100" : "opacity-0",
                    )}
                  />
                  {z}
                </span>
                <span className="text-xs text-muted-foreground">
                  {offsetLabel(z)}
                </span>
              </button>
            ))}
          </div>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
