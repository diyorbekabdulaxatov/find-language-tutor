"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import {
  ChevronDown,
  ChevronUp,
  Film,
  FileText,
  Pencil,
  Plus,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { ResourceError, listResources, uploadFile } from "@/features/resources/api";
import type { Resource } from "@/features/resources/types";
import {
  CourseError,
  addItem,
  addSection,
  deleteItem,
  deleteSection,
  renameItem,
  renameSection,
  reorderItems,
  reorderSections,
} from "@/features/courses/api";
import type { CourseDetail, CourseItem, CourseSection } from "@/features/courses/types";
import { useFileObjectUrl } from "../use-file-object-url";

function byPosition<T extends { position: number }>(items: T[]): T[] {
  return [...items].sort((a, b) => a.position - b.position);
}

function swap<T>(arr: T[], i: number, j: number): T[] {
  const next = [...arr];
  [next[i], next[j]] = [next[j], next[i]];
  return next;
}

function errMsg(err: unknown, fallback: string): string {
  return err instanceof CourseError || err instanceof ResourceError ? err.message : fallback;
}

/**
 * The sections/items tree. Every mutation call returns the full CourseDetail,
 * so this always renders from `course` and reports the fresh copy up via
 * `onCourseChange` rather than tracking its own duplicate state.
 */
export function CurriculumBuilder({
  course,
  onCourseChange,
}: {
  course: CourseDetail;
  onCourseChange: (course: CourseDetail) => void;
}) {
  const [newSectionTitle, setNewSectionTitle] = useState("");
  const [addingSection, setAddingSection] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const t = useTranslations("courses");

  // The teacher's own published resources, loaded once for the "use a
  // resource" item picker. Also gives us a title to prefill on add, since an
  // item's own title only exists once an override has been set.
  const [resources, setResources] = useState<Resource[] | null>(null);
  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const { resources: list } = await listResources({ status: "published" });
        if (alive) setResources(list);
      } catch {
        if (alive) setResources([]);
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, []);

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      onCourseChange(await fn());
    } catch (err) {
      setError(errMsg(err, t("somethingWrong")));
    } finally {
      setBusy(false);
    }
  }

  async function handleAddSection(e: React.FormEvent) {
    e.preventDefault();
    if (!newSectionTitle.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const updated = await addSection(course.id, newSectionTitle.trim());
      onCourseChange(updated);
      setNewSectionTitle("");
      setAddingSection(false);
    } catch (err) {
      setError(errMsg(err, t("couldNotAddSection")));
    } finally {
      setBusy(false);
    }
  }

  const sections = byPosition(course.sections);

  function moveSection(index: number, dir: -1 | 1) {
    const target = index + dir;
    if (target < 0 || target >= sections.length) return;
    const next = swap(sections, index, target).map((s) => s.id);
    void run(() => reorderSections(course.id, next));
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="font-display text-xl">{t("curriculum")}</h2>
        <p className="text-xs text-muted-foreground">{t("sectionCount", { count: sections.length })}</p>
      </div>

      {error && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      {sections.length === 0 && !addingSection && (
        <p className="rounded-2xl border border-border bg-card px-4 py-8 text-center text-sm text-muted-foreground">
          {t("noSections")}
        </p>
      )}

      <div className="flex flex-col gap-3">
        {sections.map((section, i) => (
          <SectionCard
            key={section.id}
            course={course}
            section={section}
            resources={resources}
            busy={busy}
            isFirst={i === 0}
            isLast={i === sections.length - 1}
            onMove={(dir) => moveSection(i, dir)}
            onCourseChange={onCourseChange}
            setBusy={setBusy}
            setError={setError}
          />
        ))}
      </div>

      {addingSection ? (
        <form onSubmit={handleAddSection} className="flex items-center gap-2">
          <Input
            autoFocus
            placeholder={t("sectionTitle")}
            value={newSectionTitle}
            onChange={(e) => setNewSectionTitle(e.target.value)}
          />
          <Button type="submit" size="sm" disabled={busy || !newSectionTitle.trim()}>
            {t("add")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              setAddingSection(false);
              setNewSectionTitle("");
            }}
          >
            {t("cancel")}
          </Button>
        </form>
      ) : (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() => setAddingSection(true)}
        >
          <Plus className="size-4" />
          {t("addSection")}
        </Button>
      )}
    </div>
  );
}

function SectionCard({
  course,
  section,
  resources,
  busy,
  isFirst,
  isLast,
  onMove,
  onCourseChange,
  setBusy,
  setError,
}: {
  course: CourseDetail;
  section: CourseSection;
  resources: Resource[] | null;
  busy: boolean;
  isFirst: boolean;
  isLast: boolean;
  onMove: (dir: -1 | 1) => void;
  onCourseChange: (course: CourseDetail) => void;
  setBusy: (b: boolean) => void;
  setError: (e: string | null) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(section.title);
  const t = useTranslations("courses");

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      onCourseChange(await fn());
    } catch (err) {
      setError(errMsg(err, t("somethingWrong")));
    } finally {
      setBusy(false);
    }
  }

  async function saveTitle() {
    const trimmed = title.trim();
    setEditing(false);
    if (!trimmed || trimmed === section.title) {
      setTitle(section.title);
      return;
    }
    void run(() => renameSection(course.id, section.id, trimmed));
  }

  async function handleDeleteSection() {
    if (
      !confirm(
        section.items.length > 0
          ? t("confirmDeleteSectionItems", { title: section.title, count: section.items.length })
          : t("confirmDeleteSection", { title: section.title }),
      )
    ) {
      return;
    }
    void run(() => deleteSection(course.id, section.id));
  }

  const items = byPosition(section.items);

  function moveItem(index: number, dir: -1 | 1) {
    const target = index + dir;
    if (target < 0 || target >= items.length) return;
    const next = swap(items, index, target).map((it) => it.id);
    void run(() => reorderItems(course.id, section.id, next));
  }

  return (
    <div className="rounded-2xl border border-border bg-card p-4">
      <div className="flex items-center gap-2">
        <div className="flex flex-col">
          <button
            type="button"
            aria-label={t("moveSectionUp")}
            disabled={isFirst || busy}
            onClick={() => onMove(-1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronUp className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label={t("moveSectionDown")}
            disabled={isLast || busy}
            onClick={() => onMove(1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronDown className="size-3.5" />
          </button>
        </div>

        {editing ? (
          <Input
            autoFocus
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={() => void saveTitle()}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void saveTitle();
              }
              if (e.key === "Escape") {
                setTitle(section.title);
                setEditing(false);
              }
            }}
            className="flex-1"
          />
        ) : (
          <button
            type="button"
            className="flex flex-1 items-center gap-1.5 truncate text-left font-medium hover:text-primary"
            onClick={() => setEditing(true)}
          >
            {section.title}
            <Pencil className="size-3 shrink-0 text-muted-foreground" />
          </button>
        )}

        <button
          type="button"
          aria-label={t("deleteSection")}
          disabled={busy}
          onClick={() => void handleDeleteSection()}
          className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
        >
          <Trash2 className="size-4" />
        </button>
      </div>

      <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
        {items.length === 0 && (
          <p className="text-xs text-muted-foreground">{t("noItems")}</p>
        )}
        {items.map((item, i) => (
          <ItemRow
            key={item.id}
            course={course}
            section={section}
            item={item}
            resources={resources}
            busy={busy}
            isFirst={i === 0}
            isLast={i === items.length - 1}
            onMove={(dir) => moveItem(i, dir)}
            onCourseChange={onCourseChange}
            setBusy={setBusy}
            setError={setError}
          />
        ))}

        <AddItemForm
          course={course}
          section={section}
          resources={resources}
          onCourseChange={onCourseChange}
          setError={setError}
        />
      </div>
    </div>
  );
}

function ItemRow({
  course,
  section,
  item,
  resources,
  busy,
  isFirst,
  isLast,
  onMove,
  onCourseChange,
  setBusy,
  setError,
}: {
  course: CourseDetail;
  section: CourseSection;
  item: CourseItem;
  resources: Resource[] | null;
  busy: boolean;
  isFirst: boolean;
  isLast: boolean;
  onMove: (dir: -1 | 1) => void;
  onCourseChange: (course: CourseDetail) => void;
  setBusy: (b: boolean) => void;
  setError: (e: string | null) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(item.title);
  const [previewing, setPreviewing] = useState(false);
  const t = useTranslations("courses");

  const linkedResource = item.kind === "resource" ? resources?.find((r) => r.id === item.resourceId) : undefined;
  const displayTitle =
    item.title || linkedResource?.title || (item.kind === "video" ? t("untitledVideo") : t("untitledResource"));

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      onCourseChange(await fn());
    } catch (err) {
      setError(errMsg(err, t("somethingWrong")));
    } finally {
      setBusy(false);
    }
  }

  async function saveTitle() {
    const trimmed = title.trim();
    setEditing(false);
    if (trimmed === item.title) return;
    void run(() => renameItem(course.id, section.id, item.id, trimmed));
  }

  async function handleDelete() {
    if (!confirm(t("confirmRemoveItem", { title: displayTitle }))) return;
    void run(() => deleteItem(course.id, section.id, item.id));
  }

  return (
    <div className="rounded-lg border border-border/60 bg-background/60 px-3 py-2">
      <div className="flex items-center gap-2">
        <div className="flex flex-col">
          <button
            type="button"
            aria-label={t("moveItemUp")}
            disabled={isFirst || busy}
            onClick={() => onMove(-1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronUp className="size-3" />
          </button>
          <button
            type="button"
            aria-label={t("moveItemDown")}
            disabled={isLast || busy}
            onClick={() => onMove(1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronDown className="size-3" />
          </button>
        </div>

        <span className="grid size-6 shrink-0 place-items-center rounded bg-muted text-muted-foreground">
          {item.kind === "video" ? <Film className="size-3.5" /> : <FileText className="size-3.5" />}
        </span>

        {editing ? (
          <Input
            autoFocus
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onBlur={() => void saveTitle()}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void saveTitle();
              }
              if (e.key === "Escape") {
                setTitle(item.title);
                setEditing(false);
              }
            }}
            className="h-7 flex-1 text-sm"
          />
        ) : (
          <button
            type="button"
            className="flex flex-1 items-center gap-1.5 truncate text-left text-sm hover:text-primary"
            onClick={() => setEditing(true)}
          >
            <span className="truncate">{displayTitle}</span>
            <Pencil className="size-3 shrink-0 text-muted-foreground" />
          </button>
        )}

        {item.kind === "video" && (
          <button
            type="button"
            className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
            onClick={() => setPreviewing((p) => !p)}
          >
            {previewing ? t("hide") : t("preview")}
          </button>
        )}
        {item.kind === "resource" && item.resourceId && (
          <Link
            href={`/resources/${item.resourceId}/edit`}
            className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
          >
            {t("open")}
          </Link>
        )}

        <button
          type="button"
          aria-label={t("deleteItem")}
          disabled={busy}
          onClick={() => void handleDelete()}
          className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
        >
          <Trash2 className="size-3.5" />
        </button>
      </div>

      {previewing && item.kind === "video" && item.videoAssetId && (
        <VideoPreview fileAssetId={item.videoAssetId} />
      )}
    </div>
  );
}

function VideoPreview({ fileAssetId }: { fileAssetId: string }) {
  const { url, state } = useFileObjectUrl(fileAssetId);
  const t = useTranslations("courses");
  return (
    <div className="mt-2">
      {state === "error" && <p className="text-xs text-destructive">{t("couldNotLoadVideoPreview")}</p>}
      {state !== "error" && !url && <div className="h-40 animate-pulse rounded-lg bg-muted" />}
      {url && <video controls src={url} className="max-h-64 w-full rounded-lg bg-black" />}
    </div>
  );
}

function AddItemForm({
  course,
  section,
  resources,
  onCourseChange,
  setError,
}: {
  course: CourseDetail;
  section: CourseSection;
  resources: Resource[] | null;
  onCourseChange: (course: CourseDetail) => void;
  setError: (e: string | null) => void;
}) {
  const [mode, setMode] = useState<"closed" | "video" | "resource">("closed");
  const [uploading, setUploading] = useState(false);
  const [selectedResourceId, setSelectedResourceId] = useState<string>("");
  const [adding, setAdding] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const t = useTranslations("courses");

  async function handleVideoFile(file: File) {
    setError(null);
    setUploading(true);
    try {
      const up = await uploadFile(file);
      const updated = await addItem(course.id, section.id, {
        kind: "video",
        title: up.filename,
        videoAssetId: up.id,
      });
      onCourseChange(updated);
      setMode("closed");
    } catch (err) {
      setError(errMsg(err, t("couldNotAddVideo")));
    } finally {
      setUploading(false);
    }
  }

  async function handleAddResource() {
    if (!selectedResourceId) return;
    const resource = resources?.find((r) => r.id === selectedResourceId);
    setAdding(true);
    setError(null);
    try {
      const updated = await addItem(course.id, section.id, {
        kind: "resource",
        title: resource?.title,
        resourceId: selectedResourceId,
      });
      onCourseChange(updated);
      setMode("closed");
      setSelectedResourceId("");
    } catch (err) {
      setError(errMsg(err, t("couldNotAddResource")));
    } finally {
      setAdding(false);
    }
  }

  if (mode === "closed") {
    return (
      <div className="mt-1 flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => setMode("video")}>
          <Upload className="size-3.5" />
          {t("uploadVideo")}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => setMode("resource")}>
          <FileText className="size-3.5" />
          {t("useResource")}
        </Button>
      </div>
    );
  }

  return (
    <div className="mt-1 flex flex-wrap items-center gap-2 rounded-lg border border-dashed border-border p-2">
      {mode === "video" && (
        <>
          <input
            ref={fileInputRef}
            type="file"
            accept="video/*"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void handleVideoFile(f);
              e.target.value = "";
            }}
          />
          <Button
            type="button"
            size="sm"
            disabled={uploading}
            onClick={() => fileInputRef.current?.click()}
          >
            {uploading ? t("uploading") : t("chooseVideoFile")}
          </Button>
          <span className="text-xs text-muted-foreground">{t("upTo500")}</span>
        </>
      )}
      {mode === "resource" &&
        (resources && resources.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            {t.rich("noPublishedResources", {
              link: (chunks) => (
                <Link href="/resources/new" className="underline">
                  {chunks}
                </Link>
              ),
            })}
          </p>
        ) : (
          <>
            <Select value={selectedResourceId} onValueChange={setSelectedResourceId}>
              <SelectTrigger className="w-56">
                <SelectValue placeholder={resources ? t("chooseResource") : t("loading")} />
              </SelectTrigger>
              <SelectContent>
                {(resources ?? []).map((r) => (
                  <SelectItem key={r.id} value={r.id}>
                    {r.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type="button"
              size="sm"
              disabled={!selectedResourceId || adding}
              onClick={() => void handleAddResource()}
            >
              {t("add")}
            </Button>
          </>
        ))}
      <button
        type="button"
        aria-label={t("cancel")}
        className={cn("ml-auto rounded p-1 text-muted-foreground hover:bg-muted", uploading && "pointer-events-none opacity-50")}
        onClick={() => setMode("closed")}
      >
        <X className="size-4" />
      </button>
    </div>
  );
}
