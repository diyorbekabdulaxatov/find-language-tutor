"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { ArrowLeft, CheckCircle2, Circle, FileText, Film } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ResourcePlayer } from "@/features/submissions/components/resource-player";
import type { AttachedResource } from "@/features/resources/types";
import { CourseError, getCourseLearn, recordCourseItemProgress } from "@/features/courses/api";
import type {
  CourseItemProgress,
  CourseLearnDetail,
  CourseLearnItem,
  CourseResourceView,
} from "@/features/courses/types";
import { useFileObjectUrl } from "../use-file-object-url";

/** Save at most this often while a video is playing, plus always on pause/end. */
const PROGRESS_SAVE_MS = 5000;

function toAttachedResource(view: CourseResourceView): AttachedResource {
  return {
    id: view.id,
    resourceId: view.id,
    kind: "homework",
    position: 0,
    dueAt: null,
    type: view.type,
    title: view.title,
    instructions: view.instructions,
    resourceStatus: "published",
    content: view.content,
    submission: null,
  };
}

function flatten(learn: CourseLearnDetail): CourseLearnItem[] {
  return learn.sections.flatMap((s) => s.items);
}

export function CoursePlayer({ id }: { id: string }) {
  const [learn, setLearn] = useState<CourseLearnDetail | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "forbidden" | "not-found" | "error">(
    "loading",
  );
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    async function load() {
      try {
        const l = await getCourseLearn(id);
        if (!alive) return;
        setLearn(l);
        setSelectedId(flatten(l)[0]?.id ?? null);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        const status = err instanceof CourseError ? err.status : undefined;
        setState(status === 403 ? "forbidden" : status === 404 ? "not-found" : "error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [id]);

  function updateItemProgress(itemId: string, progress: CourseItemProgress) {
    setLearn((prev) =>
      prev
        ? {
            ...prev,
            sections: prev.sections.map((s) => ({
              ...s,
              items: s.items.map((it) => (it.id === itemId ? { ...it, progress } : it)),
            })),
          }
        : prev,
    );
  }

  if (state === "loading") {
    return (
      <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6">
        <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
          <div className="h-96 animate-pulse rounded-2xl bg-muted" />
          <div className="h-96 animate-pulse rounded-2xl bg-muted" />
        </div>
      </div>
    );
  }

  if (state === "forbidden") {
    return (
      <EmptyState
        title="You don't have access to this course"
        body="Enroll in this course to unlock its lessons."
        href="/courses/catalog"
        cta="Browse courses"
      />
    );
  }

  if (state === "not-found") {
    return (
      <EmptyState
        title="Course not found"
        body="It may have been removed, or the link is wrong."
        href="/learn"
        cta="My learning"
      />
    );
  }

  if (state === "error" || !learn) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load this course. Please try again.
        </p>
      </div>
    );
  }

  const items = flatten(learn);
  const selected = items.find((it) => it.id === selectedId) ?? items[0] ?? null;
  const completedCount = items.filter((it) => it.progress.status === "completed").length;

  return (
    <div className="mx-auto max-w-6xl px-4 py-6 sm:px-6">
      <Link
        href="/learn"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <ArrowLeft className="size-4" />
        My learning
      </Link>

      <h1 className="mt-3 font-display text-2xl sm:text-3xl">{learn.title}</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        {completedCount} of {items.length} lessons completed
      </p>

      <div className="mt-6 grid gap-6 lg:grid-cols-[280px_1fr] lg:items-start">
        <nav className="order-2 flex flex-col gap-4 rounded-2xl bg-card p-4 ring-1 ring-border shadow-soft lg:order-1 lg:sticky lg:top-20">
          {learn.sections.map((section) => (
            <div key={section.id}>
              <p className="px-1 text-xs font-semibold text-muted-foreground">{section.title}</p>
              <ul className="mt-1.5 flex flex-col gap-0.5">
                {section.items.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      onClick={() => setSelectedId(item.id)}
                      className={cn(
                        "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition-colors",
                        item.id === selected?.id
                          ? "bg-accent text-accent-foreground"
                          : "text-foreground hover:bg-muted",
                      )}
                    >
                      {item.progress.status === "completed" ? (
                        <CheckCircle2 className="size-4 shrink-0 text-mint" />
                      ) : (
                        <Circle className="size-4 shrink-0 text-muted-foreground" />
                      )}
                      {item.kind === "video" ? (
                        <Film className="size-3.5 shrink-0 text-muted-foreground" />
                      ) : (
                        <FileText className="size-3.5 shrink-0 text-muted-foreground" />
                      )}
                      <span className="truncate">
                        {item.title || (item.kind === "video" ? "Video" : "Resource")}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>

        <div className="order-1 lg:order-2">
          {selected ? (
            <CourseItemView
              key={selected.id}
              courseId={id}
              item={selected}
              enrollmentId={learn.enrollmentId}
              onProgress={(p) => updateItemProgress(selected.id, p)}
            />
          ) : (
            <div className="rounded-2xl border border-dashed border-border bg-card p-12 text-center text-sm text-muted-foreground">
              This course has no lessons yet.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function EmptyState({
  title,
  body,
  href,
  cta,
}: {
  title: string;
  body: string;
  href: string;
  cta: string;
}) {
  return (
    <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
      <p className="font-display text-2xl">{title}</p>
      <p className="mt-2 text-sm text-muted-foreground">{body}</p>
      <Button asChild className="mt-6">
        <Link href={href}>{cta}</Link>
      </Button>
    </div>
  );
}

function CourseItemView({
  courseId,
  item,
  enrollmentId,
  onProgress,
}: {
  courseId: string;
  item: CourseLearnItem;
  enrollmentId: string | null;
  onProgress: (progress: CourseItemProgress) => void;
}) {
  if (item.kind === "resource" && item.resource) {
    return (
      <div className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
        <h2 className="font-display text-xl">{item.title || item.resource.title}</h2>
        <div className="mt-4">
          <ResourcePlayer
            attachment={toAttachedResource(item.resource)}
            enrollmentId={enrollmentId ?? undefined}
          />
        </div>
      </div>
    );
  }

  if (item.kind === "video" && item.videoAssetId) {
    return (
      <VideoItem
        courseId={courseId}
        item={item}
        videoAssetId={item.videoAssetId}
        onProgress={onProgress}
      />
    );
  }

  return (
    <div className="rounded-2xl border border-dashed border-border bg-card p-12 text-center text-sm text-muted-foreground">
      This lesson has no content yet.
    </div>
  );
}

function VideoItem({
  courseId,
  item,
  videoAssetId,
  onProgress,
}: {
  courseId: string;
  item: CourseLearnItem;
  videoAssetId: string;
  onProgress: (progress: CourseItemProgress) => void;
}) {
  const { url, state } = useFileObjectUrl(videoAssetId);
  const videoRef = useRef<HTMLVideoElement>(null);
  const lastSaveRef = useRef(0);
  const seekedRef = useRef(false);
  const [marking, setMarking] = useState(false);
  const completed = item.progress.status === "completed";

  // Resume from the last saved position once metadata is available. Guarded
  // by a ref (not state) so it fires exactly once per mounted <video>.
  function handleLoadedMetadata() {
    const video = videoRef.current;
    if (!video || seekedRef.current) return;
    seekedRef.current = true;
    if (item.progress.videoPositionSeconds > 0 && item.progress.videoPositionSeconds < video.duration) {
      video.currentTime = item.progress.videoPositionSeconds;
    }
  }

  async function saveProgress(positionSeconds: number, markComplete?: boolean) {
    try {
      const p = await recordCourseItemProgress(courseId, item.id, {
        positionSeconds: Math.floor(positionSeconds),
        completed: markComplete,
      });
      onProgress(p);
    } catch {
      // best-effort; the next tick or an explicit "mark complete" will retry
    }
  }

  function handleTimeUpdate() {
    const video = videoRef.current;
    if (!video) return;
    const now = Date.now();
    if (now - lastSaveRef.current < PROGRESS_SAVE_MS) return;
    lastSaveRef.current = now;
    void saveProgress(video.currentTime);
  }

  function handlePause() {
    const video = videoRef.current;
    if (!video) return;
    void saveProgress(video.currentTime);
  }

  function handleEnded() {
    const video = videoRef.current;
    void saveProgress(video?.duration ?? item.progress.videoPositionSeconds, true);
  }

  async function handleMarkComplete() {
    setMarking(true);
    await saveProgress(videoRef.current?.currentTime ?? item.progress.videoPositionSeconds, true);
    setMarking(false);
  }

  return (
    <div className="rounded-2xl bg-card p-6 ring-1 ring-border shadow-soft">
      <div className="flex items-start justify-between gap-3">
        <h2 className="font-display text-xl">{item.title || "Video"}</h2>
        {completed && (
          <span className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-mint/12 px-2.5 py-1 text-xs font-semibold text-mint">
            <CheckCircle2 className="size-3.5" />
            Completed
          </span>
        )}
      </div>

      <div className="mt-4 overflow-hidden rounded-xl bg-black">
        {state === "error" && (
          <p className="p-6 text-center text-sm text-destructive">Could not load this video.</p>
        )}
        {state !== "error" && !url && <div className="aspect-video animate-pulse bg-muted" />}
        {url && (
          <video
            ref={videoRef}
            src={url}
            controls
            className="aspect-video w-full"
            onLoadedMetadata={handleLoadedMetadata}
            onTimeUpdate={handleTimeUpdate}
            onPause={handlePause}
            onEnded={handleEnded}
          >
            <track kind="captions" />
          </video>
        )}
      </div>

      <div className="mt-4 flex items-center gap-3">
        <Button
          variant={completed ? "outline" : "default"}
          size="sm"
          disabled={marking || completed}
          onClick={() => void handleMarkComplete()}
        >
          {completed ? "Completed" : marking ? "Saving…" : "Mark as complete"}
        </Button>
      </div>
    </div>
  );
}
