"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowLeft, CheckCircle2, ChevronDown, FileText, PlaySquare } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { formatLectureLength } from "@/lib/format";
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
  const t = useTranslations("courses");

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
      <div>
        <div className="h-14 bg-ink" />
        <div className="grid lg:grid-cols-[minmax(0,1fr)_380px]">
          <div className="aspect-video animate-pulse bg-muted" />
          <div className="h-96 animate-pulse bg-muted/60" />
        </div>
      </div>
    );
  }

  if (state === "forbidden") {
    return (
      <EmptyState
        title={t("noAccessTitle")}
        body={t("noAccessBody")}
        href="/courses/catalog"
        cta={t("browseCourses")}
      />
    );
  }

  if (state === "not-found") {
    return (
      <EmptyState
        title={t("notFound")}
        body={t("notFoundBodyLearn")}
        href="/learn"
        cta={t("myLearning")}
      />
    );
  }

  if (state === "error" || !learn) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-24 text-center sm:px-6">
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {t("couldNotLoad")}
        </p>
      </div>
    );
  }

  const items = flatten(learn);
  const selected = items.find((it) => it.id === selectedId) ?? items[0] ?? null;
  const completedCount = items.filter((it) => it.progress.status === "completed").length;

  return (
    <div className="flex min-h-[calc(100vh-72px)] flex-col">
      {/* Udemy's course-taking top bar: wordmark, title, progress. */}
      <div className="flex h-14 items-center gap-4 bg-ink px-4 text-ink-foreground sm:px-6">
        <Link href="/" className="hidden font-display text-lg text-white sm:block" aria-label="FindTutor">
          FindTutor
        </Link>
        <span className="hidden h-6 w-px bg-white/25 sm:block" aria-hidden />
        <h1 className="min-w-0 flex-1 truncate text-sm font-bold sm:text-base">{learn.title}</h1>
        <p className="shrink-0 text-xs text-white/80 sm:text-sm">
          {t("completedOf", { completed: completedCount, total: items.length })}
        </p>
        <Link
          href="/learn"
          className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-white px-2.5 py-1.5 text-xs font-bold text-white transition-colors hover:bg-white/10 sm:text-sm"
        >
          <ArrowLeft className="size-4" />
          <span className="hidden sm:inline">{t("myLearning")}</span>
        </Link>
      </div>

      <div className="grid flex-1 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div className="min-w-0">
          {selected ? (
            <CourseItemView
              key={selected.id}
              courseId={id}
              item={selected}
              enrollmentId={learn.enrollmentId}
              onProgress={(p) => updateItemProgress(selected.id, p)}
            />
          ) : (
            <div className="m-6 border border-dashed border-border p-12 text-center text-sm text-muted-foreground">
              {t("noLessons")}
            </div>
          )}
        </div>

        {/* Course content sidebar — Udemy's right rail. */}
        <nav className="border-t border-border lg:sticky lg:top-[72px] lg:max-h-[calc(100vh-72px)] lg:overflow-y-auto lg:border-t-0 lg:border-l">
          <p className="border-b border-border px-4 py-3 text-base font-bold">{t("courseContent")}</p>
          {learn.sections.map((section, i) => {
            const done = section.items.filter((it) => it.progress.status === "completed").length;
            const secs = section.items.reduce((n, it) => n + it.durationSeconds, 0);
            return (
              <PlayerSection
                key={section.id}
                title={t("sectionN", { n: i + 1, title: section.title })}
                meta={[`${done} / ${section.items.length}`, formatLectureLength(secs)].filter(Boolean).join(" | ")}
                defaultOpen={section.items.some((it) => it.id === selected?.id) || i === 0}
              >
                {section.items.map((item) => {
                  const active = item.id === selected?.id;
                  const completed = item.progress.status === "completed";
                  const len = formatLectureLength(item.durationSeconds);
                  return (
                    <li key={item.id}>
                      <button
                        type="button"
                        onClick={() => setSelectedId(item.id)}
                        aria-current={active ? "true" : undefined}
                        className={cn(
                          "flex w-full items-start gap-3 px-4 py-2.5 text-left text-sm transition-colors",
                          active ? "bg-border/60" : "hover:bg-muted",
                        )}
                      >
                        <span
                          aria-hidden
                          className={cn(
                            "mt-0.5 grid size-4 shrink-0 place-items-center border border-foreground",
                            completed && "bg-ink text-ink-foreground",
                          )}
                        >
                          {completed && <CheckCircle2 className="size-3" />}
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-foreground">
                            {item.title || (item.kind === "video" ? t("video") : t("resource"))}
                          </span>
                          <span className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground">
                            {item.kind === "video" ? (
                              <PlaySquare className="size-3.5" />
                            ) : (
                              <FileText className="size-3.5" />
                            )}
                            {len ?? (item.kind === "video" ? t("video") : t("resource"))}
                          </span>
                        </span>
                      </button>
                    </li>
                  );
                })}
              </PlayerSection>
            );
          })}
        </nav>
      </div>
    </div>
  );
}

function PlayerSection({
  title,
  meta,
  defaultOpen,
  children,
}: {
  title: string;
  meta: string;
  defaultOpen: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <div className="border-b border-border">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-start gap-3 bg-muted px-4 py-3 text-left"
      >
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-bold text-foreground">{title}</span>
          <span className="block text-xs text-muted-foreground">{meta}</span>
        </span>
        <ChevronDown className={cn("mt-1 size-4 shrink-0 transition-transform", open && "rotate-180")} />
      </button>
      {open && <ul>{children}</ul>}
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
  const t = useTranslations("courses");
  if (item.kind === "resource" && item.resource) {
    return (
      <div className="px-4 py-6 sm:px-8">
        <h2 className="font-display text-2xl">{item.title || item.resource.title}</h2>
        <div className="mt-4">
          <ResourcePlayer
            attachment={toAttachedResource(item.resource)}
            enrollmentId={enrollmentId ?? undefined}
            previewOnly={!enrollmentId}
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
    <div className="m-6 border border-dashed border-border p-12 text-center text-sm text-muted-foreground">
      {t("noContent")}
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
  const saveSeqRef = useRef(0);
  const seekedRef = useRef(false);
  const [marking, setMarking] = useState(false);
  const completed = item.progress.status === "completed";
  const t = useTranslations("courses");

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
    const seq = ++saveSeqRef.current;
    try {
      const p = await recordCourseItemProgress(courseId, item.id, {
        positionSeconds: Math.floor(positionSeconds),
        completed: markComplete,
      });
      // Drop a response that lost the race to a save started after it — e.g.
      // `pause` then `ended` fire back-to-back, and the `pause` request (no
      // `completed` flag) must never clobber `ended`'s "completed" result if
      // it resolves later.
      if (seq !== saveSeqRef.current) return;
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
    <div>
      <div className="bg-black">
        {state === "error" && (
          <p className="p-6 text-center text-sm text-destructive">{t("couldNotLoadVideo")}</p>
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

      <div className="px-4 sm:px-8">
        <div className="flex gap-6 border-b border-border text-base font-bold">
          <span className="-mb-px border-b-2 border-foreground py-3">{t("overview")}</span>
        </div>
        <div className="flex flex-wrap items-start justify-between gap-4 py-6">
          <h2 className="font-display text-2xl">{item.title || t("video")}</h2>
          <Button
            variant={completed ? "outline" : "default"}
            disabled={marking || completed}
            onClick={() => void handleMarkComplete()}
          >
            {completed && <CheckCircle2 />}
            {completed ? t("completed") : marking ? t("saving") : t("markComplete")}
          </Button>
        </div>
      </div>
    </div>
  );
}
