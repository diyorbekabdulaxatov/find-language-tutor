"use client";

import { useTheme } from "next-themes";
import { Monitor, Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";

const ORDER = ["system", "light", "dark"] as const;
type Choice = (typeof ORDER)[number];

const NEXT_LABEL: Record<Choice, string> = {
  system: "Switch to light theme",
  light: "Switch to dark theme",
  dark: "Match system theme",
};

/**
 * Cycles system -> light -> dark.
 *
 * next-themes returns `theme: undefined` during SSR and the first client render,
 * so `current` resolves to "system" in both — no hydration mismatch, no need for
 * a mounted flag. The icon may flip once just after mount if a non-system theme
 * was stored; that's a single frame and acceptable.
 */
export function ThemeToggle() {
  const { theme, setTheme } = useTheme();

  const current: Choice = ORDER.includes(theme as Choice)
    ? (theme as Choice)
    : "system";

  const Icon = current === "system" ? Monitor : current === "light" ? Sun : Moon;

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={NEXT_LABEL[current]}
      title={NEXT_LABEL[current]}
      onClick={() =>
        setTheme(ORDER[(ORDER.indexOf(current) + 1) % ORDER.length])
      }
    >
      <Icon />
    </Button>
  );
}
