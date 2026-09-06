"use client";

import { useSyncExternalStore } from "react";
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
 * `useSyncExternalStore` gives a lint-clean "am I hydrated yet" flag: it returns
 * the server snapshot (false) during SSR and hydration, then the client snapshot
 * (true). Until then we render a blank icon slot so the button markup matches
 * what the server sent — the stored theme is only known on the client.
 */
const subscribe = () => () => {};
function useMounted() {
  return useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
}

/** Cycles system -> light -> dark. */
export function ThemeToggle() {
  const mounted = useMounted();
  const { theme, setTheme } = useTheme();

  const current: Choice =
    mounted && ORDER.includes(theme as Choice) ? (theme as Choice) : "system";
  const Icon = current === "system" ? Monitor : current === "light" ? Sun : Moon;

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={mounted ? NEXT_LABEL[current] : "Toggle theme"}
      title={mounted ? NEXT_LABEL[current] : undefined}
      onClick={() => setTheme(ORDER[(ORDER.indexOf(current) + 1) % ORDER.length])}
    >
      {mounted ? <Icon /> : <span className="size-4" />}
    </Button>
  );
}
