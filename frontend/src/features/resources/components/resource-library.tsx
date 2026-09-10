"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { FileText, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  ResourceError,
  archiveResource,
  listResources,
  publishResource,
  unarchiveResource,
  unpublishResource,
} from "@/features/resources/api";
import {
  RESOURCE_TYPES,
  type Resource,
  type ResourceStatus,
  type ResourceType,
  typeLabel,
} from "@/features/resources/types";

type TypeFilter = ResourceType | "all";
type StatusFilter = ResourceStatus | "all";

export function ResourceLibrary() {
  const [type, setType] = useState<TypeFilter>("all");
  const [status, setStatus] = useState<StatusFilter>("all");
  const [showArchived, setShowArchived] = useState(false);

  const [items, setItems] = useState<Resource[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error" | "no-teacher">("loading");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const res = await listResources({
          type: type === "all" ? undefined : type,
          status: status === "all" ? undefined : status,
          archived: showArchived,
        });
        if (!alive) return;
        setItems(res.resources);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setState(err instanceof ResourceError && err.code === "no_teacher_profile" ? "no-teacher" : "error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [type, status, showArchived, reloadKey]);

  const reload = () => setReloadKey((k) => k + 1);

  if (state === "no-teacher") {
    return (
      <div className="rounded-2xl border border-border bg-card px-6 py-12 text-center">
        <p className="text-sm text-muted-foreground">
          Resources are part of your teaching toolkit — create a teacher profile
          first.
        </p>
        <Button asChild className="mt-4">
          <Link href="/dashboard">Go to the dashboard</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-display text-3xl">Teaching resources</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Build materials and tasks once, then attach them to lessons.
          </p>
        </div>
        <Button asChild>
          <Link href="/resources/new">
            <Plus className="size-4" />
            New resource
          </Link>
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Chip active={type === "all"} onClick={() => setType("all")}>
          All types
        </Chip>
        {RESOURCE_TYPES.map((t) => (
          <Chip key={t.value} active={type === t.value} onClick={() => setType(t.value)}>
            {t.label}
          </Chip>
        ))}
        <span className="mx-1 h-4 w-px bg-border" />
        {(["all", "draft", "published"] as StatusFilter[]).map((s) => (
          <Chip key={s} active={status === s} onClick={() => setStatus(s)}>
            {s === "all" ? "Any status" : s}
          </Chip>
        ))}
        <label className="ml-1 flex items-center gap-1.5 text-sm text-muted-foreground">
          <input
            type="checkbox"
            checked={showArchived}
            onChange={(e) => setShowArchived(e.target.checked)}
            className="accent-primary"
          />
          Archived
        </label>
      </div>

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load your resources.
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse rounded-2xl bg-muted" />
      ) : items.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-12 text-center text-sm text-muted-foreground">
          Nothing here yet.
        </p>
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {items.map((r) => (
            <ResourceCard key={r.id} resource={r} onChanged={reload} />
          ))}
        </ul>
      )}
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
      onClick={onClick}
      className={cn(
        "rounded-lg border px-2.5 py-1 text-xs font-medium capitalize transition-colors",
        active ? "border-primary bg-accent" : "border-border hover:bg-muted",
      )}
    >
      {children}
    </button>
  );
}

function ResourceCard({ resource: r, onChanged }: { resource: Resource; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);

  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    try {
      await fn();
      onChanged();
    } catch {
      setBusy(false);
    }
  }

  return (
    <li className="flex flex-col gap-3 rounded-2xl border border-border bg-card p-4">
      <div className="flex items-start gap-3">
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
          <FileText className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <Link
            href={`/resources/${r.id}/edit`}
            className="block truncate font-medium hover:text-primary hover:underline"
          >
            {r.title}
          </Link>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {typeLabel(r.type)}
            {(r.type === "quiz" || r.type === "listening" || r.type === "reading") &&
              (() => {
                const n = r.content.questions?.length ?? 0;
                return ` · ${n} question${n === 1 ? "" : "s"}`;
              })()}
          </p>
        </div>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold",
            r.archived
              ? "bg-muted text-muted-foreground"
              : r.status === "published"
                ? "bg-primary/15 text-primary"
                : "bg-star/15 text-star",
          )}
        >
          {r.archived ? "Archived" : r.status}
        </span>
      </div>

      <div className="flex flex-wrap gap-2 border-t border-border pt-3">
        <Button asChild size="sm" variant="outline">
          <Link href={`/resources/${r.id}/edit`}>Edit</Link>
        </Button>
        {!r.archived &&
          (r.status === "published" ? (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => run(() => unpublishResource(r.id))}>
              Unpublish
            </Button>
          ) : (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => run(() => publishResource(r.id))}>
              Publish
            </Button>
          ))}
        <Button
          size="sm"
          variant="ghost"
          className="ml-auto text-muted-foreground"
          disabled={busy}
          onClick={() => run(() => (r.archived ? unarchiveResource(r.id) : archiveResource(r.id)))}
        >
          {r.archived ? "Restore" : "Archive"}
        </Button>
      </div>
    </li>
  );
}
