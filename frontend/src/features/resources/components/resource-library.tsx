"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
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
  const t = useTranslations("resources");
  const tType = useTranslations("resourceTypes");

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
      <div className="border border-border bg-card px-6 py-16 text-center">
        <p className="text-sm text-muted-foreground">
          {t("needProfile")}
        </p>
        <Button asChild className="mt-4">
          <Link href="/dashboard">{t("goDashboard")}</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-display text-3xl sm:text-[2rem]">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("intro")}</p>
        </div>
        <Button asChild>
          <Link href="/resources/new">
            <Plus className="size-4" />
            {t("newResource")}
          </Link>
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Chip active={type === "all"} onClick={() => setType("all")}>
          {t("allTypes")}
        </Chip>
        {RESOURCE_TYPES.map((rt) => (
          <Chip key={rt} active={type === rt} onClick={() => setType(rt)}>
            {tType(`long.${rt}`)}
          </Chip>
        ))}
        <span className="mx-1 h-4 w-px bg-border" />
        {(["all", "draft", "published"] as StatusFilter[]).map((s) => (
          <Chip key={s} active={status === s} onClick={() => setStatus(s)}>
            {s === "all" ? t("anyStatus") : t(s)}
          </Chip>
        ))}
        <label className="ml-1 flex items-center gap-1.5 text-sm text-muted-foreground">
          <input
            type="checkbox"
            checked={showArchived}
            onChange={(e) => setShowArchived(e.target.checked)}
            className="accent-primary"
          />
          {t("archived")}
        </label>
      </div>

      {state === "error" ? (
        <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoad")}
        </p>
      ) : state === "loading" ? (
        <div className="h-64 animate-pulse bg-muted" />
      ) : items.length === 0 ? (
        <p className="border border-border bg-card px-4 py-16 text-center text-sm text-muted-foreground">
          {t("nothingYet")}
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
        "h-8 rounded-md border px-3 text-xs font-bold transition-colors",
        active ? "border-foreground bg-foreground text-background" : "border-border hover:bg-accent",
      )}
    >
      {children}
    </button>
  );
}

function ResourceCard({ resource: r, onChanged }: { resource: Resource; onChanged: () => void }) {
  const [busy, setBusy] = useState(false);
  const t = useTranslations("resources");
  const tType = useTranslations("resourceTypes");

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
    <li className="flex flex-col gap-3 border border-border bg-card p-4">
      <div className="flex items-start gap-3">
        <span className="grid size-9 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">
          <FileText className="size-4" />
        </span>
        <div className="min-w-0 flex-1">
          <Link
            href={`/resources/${r.id}/edit`}
            className="block truncate font-bold hover:text-link hover:underline"
          >
            {r.title}
          </Link>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {tType(`long.${r.type}`)}
            {(r.type === "quiz" || r.type === "listening" || r.type === "reading") &&
              ` ${t("questionCount", { count: r.content.questions?.length ?? 0 })}`}
          </p>
        </div>
        <span
          className={cn(
            "shrink-0 rounded-sm px-2 py-0.5 text-xs font-bold",
            r.archived
              ? "bg-muted text-muted-foreground"
              : r.status === "published"
                ? "bg-accent text-accent-foreground"
                : "bg-star/20 text-rating dark:text-star",
          )}
        >
          {r.archived ? t("archived") : t(r.status)}
        </span>
      </div>

      <div className="flex flex-wrap gap-2 border-t border-border pt-3">
        <Button asChild size="sm" variant="outline">
          <Link href={`/resources/${r.id}/edit`}>{t("edit")}</Link>
        </Button>
        {!r.archived &&
          (r.status === "published" ? (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => run(() => unpublishResource(r.id))}>
              {t("unpublish")}
            </Button>
          ) : (
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => run(() => publishResource(r.id))}>
              {t("publish")}
            </Button>
          ))}
        <Button
          size="sm"
          variant="ghost"
          className="ml-auto text-muted-foreground"
          disabled={busy}
          onClick={() => run(() => (r.archived ? unarchiveResource(r.id) : archiveResource(r.id)))}
        >
          {r.archived ? t("restore") : t("archive")}
        </Button>
      </div>
    </li>
  );
}
