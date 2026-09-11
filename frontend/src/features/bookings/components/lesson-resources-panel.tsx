"use client";

import { useEffect, useState, type ComponentType } from "react";
import {
  BookOpen,
  BookOpenText,
  ClipboardList,
  FileText,
  Headphones,
  PenLine,
  Plus,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import type { Booking } from "@/features/bookings/api";
import { formatFull, viewerTimezone } from "@/features/bookings/datetime";
import {
  ResourceError,
  attachResourceToBooking,
  detachBookingResource,
  listBookingAttachments,
  listResources,
} from "@/features/resources/api";
import type {
  AttachedResource,
  Resource,
  ResourceKind,
  ResourceType,
  Submission,
} from "@/features/resources/types";
import { ResourcePlayer } from "@/features/submissions/components/resource-player";
import { SubmissionStatusBadge } from "@/features/submissions/components/submission-status-badge";

const TYPE_ICON: Record<ResourceType, ComponentType<{ className?: string }>> = {
  material: FileText,
  article: BookOpenText,
  quiz: ClipboardList,
  listening: Headphones,
  reading: BookOpenText,
  writing: PenLine,
};

/**
 * The booking-detail "Lesson resources" section — materials + homework
 * attached to this booking. Self-contained: its own fetch + state, mirroring
 * `DisputePanel`. Teacher-owner gets an attach/detach control; the student
 * gets the resource player for homework.
 */
export function LessonResourcesPanel({
  booking,
  isTeacher,
  isStudent,
}: {
  booking: Booking;
  isTeacher: boolean;
  isStudent: boolean;
}) {
  const [attachments, setAttachments] = useState<AttachedResource[]>([]);
  const [state, setState] = useState<"loading" | "ready" | "error">("loading");
  const [error, setError] = useState<string | null>(null);
  const tz = viewerTimezone();

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const list = await listBookingAttachments(booking.id);
        if (!alive) return;
        setAttachments(list);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setError(
          err instanceof ResourceError ? err.message : "Could not load this lesson's resources.",
        );
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [booking.id]);

  async function refresh() {
    try {
      const list = await listBookingAttachments(booking.id);
      setAttachments(list);
    } catch {
      /* keep showing the last-known list */
    }
  }

  async function handleDetach(attachmentId: string) {
    if (
      !confirm(
        "Remove this resource from the lesson? Any submission already filed against it is kept.",
      )
    ) {
      return;
    }
    setError(null);
    try {
      await detachBookingResource(booking.id, attachmentId);
      setAttachments((prev) => prev.filter((a) => a.id !== attachmentId));
    } catch (err) {
      setError(err instanceof ResourceError ? err.message : "Could not remove that resource.");
    }
  }

  if (state === "loading") {
    return <div className="mt-4 h-24 animate-pulse rounded-2xl bg-muted" />;
  }

  const materials = attachments.filter((a) => a.kind === "material");
  const homework = attachments.filter((a) => a.kind === "homework");

  // Nothing to show and nothing the viewer can do.
  if (!isTeacher && materials.length === 0 && homework.length === 0 && state !== "error") {
    return null;
  }

  return (
    <div className="mt-4 rounded-2xl border border-border bg-card p-6">
      <div className="flex items-center justify-between gap-3">
        <h2 className="inline-flex items-center gap-2 font-display text-lg">
          <BookOpen className="size-4 text-primary" /> Lesson resources
        </h2>
        {isTeacher && (
          <AttachResourceSheet bookingId={booking.id} onAttached={() => void refresh()} />
        )}
      </div>

      {error && (
        <p className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      <ResourceGroup
        title="Materials"
        items={materials}
        emptyHint={isTeacher ? "Attach a file, link or article for your student." : null}
        bookingId={booking.id}
        isTeacher={isTeacher}
        isStudent={isStudent}
        tz={tz}
        onDetach={handleDetach}
        onSubmissionChange={() => void refresh()}
      />
      <ResourceGroup
        title="Homework"
        items={homework}
        emptyHint={isTeacher ? "Assign a quiz, listening, reading or writing task." : null}
        bookingId={booking.id}
        isTeacher={isTeacher}
        isStudent={isStudent}
        tz={tz}
        onDetach={handleDetach}
        onSubmissionChange={() => void refresh()}
      />
    </div>
  );
}

function ResourceGroup({
  title,
  items,
  emptyHint,
  bookingId,
  isTeacher,
  isStudent,
  tz,
  onDetach,
  onSubmissionChange,
}: {
  title: string;
  items: AttachedResource[];
  emptyHint: string | null;
  bookingId: string;
  isTeacher: boolean;
  isStudent: boolean;
  tz: string;
  onDetach: (attachmentId: string) => void;
  onSubmissionChange: (s: Submission) => void;
}) {
  if (items.length === 0 && !emptyHint) return null;

  return (
    <div className="mt-4 border-t border-border pt-4 first:mt-3 first:border-t-0 first:pt-0">
      <h3 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {title}
      </h3>
      {items.length === 0 ? (
        <p className="mt-2 text-sm text-muted-foreground">{emptyHint}</p>
      ) : (
        <ul className="mt-2 flex flex-col gap-2">
          {items.map((a) => (
            <ResourceRow
              key={a.id}
              attachment={a}
              bookingId={bookingId}
              isTeacher={isTeacher}
              isStudent={isStudent}
              tz={tz}
              onDetach={onDetach}
              onSubmissionChange={onSubmissionChange}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function ResourceRow({
  attachment: a,
  bookingId,
  isTeacher,
  isStudent,
  tz,
  onDetach,
  onSubmissionChange,
}: {
  attachment: AttachedResource;
  bookingId: string;
  isTeacher: boolean;
  isStudent: boolean;
  tz: string;
  onDetach: (attachmentId: string) => void;
  onSubmissionChange: (s: Submission) => void;
}) {
  const [open, setOpen] = useState(false);
  const Icon = TYPE_ICON[a.type];
  const canOpen = a.kind === "material" || isStudent;

  const openLabel =
    a.kind === "material"
      ? "View"
      : !a.submission || a.submission.status === "in_progress"
        ? a.submission
          ? "Continue"
          : "Start"
        : a.submission.status === "submitted"
          ? "Review"
          : "View feedback";

  return (
    <li className="flex items-start gap-3 rounded-xl border border-border bg-background/40 p-3">
      <span className="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
        <Icon className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{a.title}</p>
        <p className="mt-0.5 text-xs text-muted-foreground">
          {a.type}
          {a.dueAt && <> · Due {formatFull(a.dueAt, tz)}</>}
        </p>
        {a.kind === "homework" && isStudent && (
          <div className="mt-1.5">
            <SubmissionStatusBadge
              status={a.submission?.status ?? ""}
              autoScore={a.submission?.autoScore}
              autoMax={a.submission?.autoMax}
              teacherScore={a.submission?.teacherScore}
            />
          </div>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {canOpen && (
          <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
              <Button size="sm" variant="outline">
                {openLabel}
              </Button>
            </SheetTrigger>
            <SheetContent side="right" className="w-full max-w-md sm:max-w-lg">
              <SheetHeader>
                <SheetTitle>{a.title}</SheetTitle>
              </SheetHeader>
              <ResourcePlayer
                attachment={a}
                bookingId={bookingId}
                onSubmissionChange={(s) => {
                  onSubmissionChange(s);
                }}
              />
            </SheetContent>
          </Sheet>
        )}
        {isTeacher && (
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label="Remove from lesson"
            className="text-muted-foreground hover:text-destructive"
            onClick={() => onDetach(a.id)}
          >
            <Trash2 className="size-4" />
          </Button>
        )}
      </div>
    </li>
  );
}

function AttachResourceSheet({
  bookingId,
  onAttached,
}: {
  bookingId: string;
  onAttached: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [resources, setResources] = useState<Resource[]>([]);
  const [loadState, setLoadState] = useState<"loading" | "ready" | "error">("loading");
  const [resourceId, setResourceId] = useState("");
  const [kind, setKind] = useState<ResourceKind>("material");
  const [dueAt, setDueAt] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    async function load() {
      setLoadState("loading");
      try {
        const { resources: list } = await listResources({ status: "published" });
        if (!alive) return;
        setResources(list);
        setLoadState("ready");
      } catch {
        if (!alive) return;
        setLoadState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [open]);

  const selected = resources.find((r) => r.id === resourceId) ?? null;
  const canHomework = selected ? selected.type !== "material" && selected.type !== "article" : false;

  function selectResource(id: string) {
    setResourceId(id);
    const r = resources.find((x) => x.id === id);
    if (r && (r.type === "material" || r.type === "article")) setKind("material");
  }

  async function submit() {
    if (!resourceId) return;
    setBusy(true);
    setError(null);
    try {
      await attachResourceToBooking(bookingId, {
        resourceId,
        kind,
        dueAt: kind === "homework" && dueAt ? new Date(dueAt).toISOString() : undefined,
      });
      setOpen(false);
      setResourceId("");
      setKind("material");
      setDueAt("");
      onAttached();
    } catch (err) {
      setError(err instanceof ResourceError ? err.message : "Could not attach that resource.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button size="sm">
          <Plus className="size-4" />
          Attach a resource
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-full max-w-md">
        <SheetHeader>
          <SheetTitle>Attach a resource</SheetTitle>
        </SheetHeader>

        {loadState === "loading" ? (
          <div className="h-40 animate-pulse rounded-xl bg-muted" />
        ) : loadState === "error" ? (
          <p className="text-sm text-destructive">Could not load your resources.</p>
        ) : resources.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            You have no published resources yet — publish one from{" "}
            <a href="/resources" className="underline">
              Teaching resources
            </a>{" "}
            first.
          </p>
        ) : (
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label htmlFor="attach-resource" className="text-sm font-medium">
                Resource
              </label>
              <select
                id="attach-resource"
                value={resourceId}
                onChange={(e) => selectResource(e.target.value)}
                className="w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              >
                <option value="">Choose a resource…</option>
                {resources.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.title} ({r.type})
                  </option>
                ))}
              </select>
            </div>

            <div className="flex flex-col gap-1.5">
              <span className="text-sm font-medium">Assign as</span>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setKind("material")}
                  className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors ${
                    kind === "material"
                      ? "border-primary bg-accent"
                      : "border-border hover:bg-muted"
                  }`}
                >
                  Material
                </button>
                <button
                  type="button"
                  disabled={!canHomework}
                  onClick={() => setKind("homework")}
                  className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
                    kind === "homework"
                      ? "border-primary bg-accent"
                      : "border-border hover:bg-muted"
                  }`}
                >
                  Homework
                </button>
              </div>
              {selected && !canHomework && (
                <p className="text-xs text-muted-foreground">
                  {selected.type === "material" ? "Materials" : "Articles"} have no submission
                  flow — attach as material.
                </p>
              )}
            </div>

            {kind === "homework" && (
              <div className="flex flex-col gap-1.5">
                <label htmlFor="attach-due" className="text-sm font-medium">
                  Due date (optional)
                </label>
                <input
                  id="attach-due"
                  type="datetime-local"
                  value={dueAt}
                  onChange={(e) => setDueAt(e.target.value)}
                  className="w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                />
              </div>
            )}

            {error && (
              <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {error}
              </p>
            )}

            <Button onClick={() => void submit()} disabled={busy || !resourceId} className="self-start">
              {busy ? "Attaching…" : "Attach"}
            </Button>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
