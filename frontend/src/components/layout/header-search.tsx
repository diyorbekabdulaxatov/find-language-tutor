"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { Search } from "lucide-react";

/**
 * The header's pill search — Udemy's "Search for anything". It searches
 * teachers (the marketplace's main catalog); the course catalog has its own
 * search on its page.
 */
export function HeaderSearch({ placeholder }: { placeholder: string }) {
  const router = useRouter();
  const [q, setQ] = useState("");

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const term = q.trim();
    router.push(term ? `/teachers?q=${encodeURIComponent(term)}` : "/teachers");
  }

  return (
    <form
      onSubmit={submit}
      role="search"
      className="hidden min-w-0 flex-1 items-center md:flex"
    >
      <label className="flex h-12 w-full items-center gap-3 rounded-full border border-foreground bg-muted px-4 transition-colors focus-within:bg-background dark:border-border">
        <Search className="size-5 shrink-0 text-muted-foreground" aria-hidden />
        <input
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={placeholder}
          aria-label={placeholder}
          className="h-full w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground [&::-webkit-search-cancel-button]:appearance-none"
        />
      </label>
    </form>
  );
}
