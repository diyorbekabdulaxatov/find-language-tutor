"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
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
      setError(errMsg(err, "Something went wrong."));
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
      setError(errMsg(err, "Could not add that section."));
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
        <h2 className="font-display text-xl">Curriculum</h2>
        <p className="text-xs text-muted-foreground">
          {sections.length} section{sections.length === 1 ? "" : "s"}
        </p>
      </div>

      {error && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      {sections.length === 0 && !addingSection && (
        <p className="rounded-2xl border border-border bg-card px-4 py-8 text-center text-sm text-muted-foreground">
          No sections yet. A course needs at least one section with an item before it can be published.
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
            placeholder="Section title"
            value={newSectionTitle}
            onChange={(e) => setNewSectionTitle(e.target.value)}
          />
          <Button type="submit" size="sm" disabled={busy || !newSectionTitle.trim()}>
            Add
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
            Cancel
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
          Add section
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

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      onCourseChange(await fn());
    } catch (err) {
      setError(errMsg(err, "Something went wrong."));
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
          ? `Delete "${section.title}" and its ${section.items.length} item${section.items.length === 1 ? "" : "s"}?`
          : `Delete "${section.title}"?`,
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
            aria-label="Move section up"
            disabled={isFirst || busy}
            onClick={() => onMove(-1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronUp className="size-3.5" />
          </button>
          <button
            type="button"
            aria-label="Move section down"
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
          aria-label="Delete section"
          disabled={busy}
          onClick={() => void handleDeleteSection()}
          className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-destructive"
        >
          <Trash2 className="size-4" />
        </button>
      </div>

      <div className="mt-3 flex flex-col gap-2 border-t border-border pt-3">
        {items.length === 0 && (
          <p className="text-xs text-muted-foreground">No items in this section yet.</p>
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

  const linkedResource = item.kind === "resource" ? resources?.find((r) => r.id === item.resourceId) : undefined;
  const displayTitle =
    item.title || linkedResource?.title || (item.kind === "video" ? "Untitled video" : "Untitled resource");

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      onCourseChange(await fn());
    } catch (err) {
      setError(errMsg(err, "Something went wrong."));
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
    if (!confirm(`Remove "${displayTitle}" from this section?`)) return;
    void run(() => deleteItem(course.id, section.id, item.id));
  }

  return (
    <div className="rounded-lg border border-border/60 bg-background/60 px-3 py-2">
      <div className="flex items-center gap-2">
        <div className="flex flex-col">
          <button
            type="button"
            aria-label="Move item up"
            disabled={isFirst || busy}
            onClick={() => onMove(-1)}
            className="rounded p-0.5 text-muted-foreground hover:bg-muted disabled:opacity-30"
          >
            <ChevronUp className="size-3" />
          </button>
          <button
            type="button"
            aria-label="Move item down"
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
            {previewing ? "Hide" : "Preview"}
          </button>
        )}
        {item.kind === "resource" && item.resourceId && (
          <Link
            href={`/resources/${item.resourceId}/edit`}
            className="shrink-0 text-xs text-muted-foreground hover:text-foreground"
          >
            Open
          </Link>
        )}

        <button
          type="button"
          aria-label="Delete item"
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
  return (
    <div className="mt-2">
      {state === "error" && <p className="text-xs text-destructive">Could not load the video.</p>}
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
      setError(errMsg(err, "Could not add that video."));
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
      setError(errMsg(err, "Could not add that resource."));
    } finally {
      setAdding(false);
    }
  }

  if (mode === "closed") {
    return (
      <div className="mt-1 flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => setMode("video")}>
          <Upload className="size-3.5" />
          Upload a video
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={() => setMode("resource")}>
          <FileText className="size-3.5" />
          Use a resource
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
            {uploading ? "Uploading…" : "Choose a video file"}
          </Button>
          <span className="text-xs text-muted-foreground">Up to 500MB.</span>
        </>
      )}
      {mode === "resource" &&
        (resources && resources.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            You have no published resources yet.{" "}
            <Link href="/resources/new" className="underline">
              Create one
            </Link>
            .
          </p>
        ) : (
          <>
            <Select value={selectedResourceId} onValueChange={setSelectedResourceId}>
              <SelectTrigger className="w-56">
                <SelectValue placeholder={resources ? "Choose a resource…" : "Loading…"} />
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
              Add
            </Button>
          </>
        ))}
      <button
        type="button"
        aria-label="Cancel"
        className={cn("ml-auto rounded p-1 text-muted-foreground hover:bg-muted", uploading && "pointer-events-none opacity-50")}
        onClick={() => setMode("closed")}
      >
        <X className="size-4" />
      </button>
    </div>
  );
}
