"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Camera, Clapperboard, Trash2 } from "lucide-react";
import type { ProfileFormValues } from "@/features/dashboard/api";
import { ProfileError } from "@/features/dashboard/api";
import { uploadWithProgress } from "@/features/dashboard/upload";
import { Field, Section } from "@/features/dashboard/components/form-bits";
import { useFileObjectUrl } from "@/features/courses/use-file-object-url";
import { initials } from "@/features/teachers/components/teacher-avatar";
import { isUploadedMedia } from "@/lib/media";
import type { TeacherModerationStatus } from "@/types/teacher";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

const MAX_IMAGE = 25 * 1024 * 1024;
const MAX_VIDEO = 500 * 1024 * 1024;

type Setter = <K extends keyof ProfileFormValues>(key: K, val: ProfileFormValues[K]) => void;

/**
 * Step 3: photo, intro video, meeting link. Uploads go straight to
 * `POST /v1/uploads`; the form only ever holds the resulting asset id, and
 * the backend derives the public URL when the profile is saved.
 */
export function MediaStep({
  values,
  set,
  profileStatus,
}: {
  values: ProfileFormValues;
  set: Setter;
  profileStatus: TeacherModerationStatus | null;
}) {
  const t = useTranslations("profileEditor");
  const approved = profileStatus === "approved";

  return (
    <div className="flex flex-col gap-8">
      <Section title={t("media")} hint={t("mediaHint")}>
        <div className="grid gap-6 md:grid-cols-[auto_1fr] md:items-start">
          <MediaSlot
            kind="image"
            assetId={values.avatarAssetId}
            currentUrl={values.avatarUrl}
            publicOk={approved}
            fallbackName={values.displayName}
            onUploaded={(id) => set("avatarAssetId", id)}
            onRemove={() => {
              set("avatarAssetId", null);
              set("avatarUrl", "");
            }}
            labels={{
              title: t("photo"),
              hint: t("photoHint"),
              choose: t("choosePhoto"),
              replace: t("replacePhoto"),
              remove: t("removePhoto"),
            }}
          />
          <MediaSlot
            kind="video"
            assetId={values.introVideoAssetId}
            currentUrl={values.introVideoUrl}
            publicOk={approved}
            fallbackName={values.displayName}
            onUploaded={(id) => set("introVideoAssetId", id)}
            onRemove={() => {
              set("introVideoAssetId", null);
              set("introVideoUrl", "");
            }}
            labels={{
              title: t("video"),
              hint: t("videoHint"),
              choose: t("chooseVideo"),
              replace: t("replaceVideo"),
              remove: t("removeVideo"),
            }}
          />
        </div>
      </Section>

      <Section title={t("videoRoom")} hint={t("videoRoomHint")}>
        <Field label={t("meetingLink")} htmlFor="meeting">
          <Input
            id="meeting"
            type="url"
            placeholder="https://meet.google.com/abc-defg-hij"
            value={values.meetingUrl}
            onChange={(e) => set("meetingUrl", e.target.value)}
          />
        </Field>
      </Section>
    </div>
  );
}

/**
 * One upload slot. What it previews, in order: the file just picked (an
 * object URL, instant), else the stored upload — the public media route when
 * the profile is approved, otherwise fetched through the authenticated file
 * endpoint (a rejected profile's media route 404s) — else a pasted URL.
 */
function MediaSlot({
  kind,
  assetId,
  currentUrl,
  publicOk,
  fallbackName,
  onUploaded,
  onRemove,
  labels,
}: {
  kind: "image" | "video";
  assetId: string | null;
  currentUrl: string;
  publicOk: boolean;
  fallbackName: string;
  onUploaded: (id: string) => void;
  onRemove: () => void;
  labels: { title: string; hint: string; choose: string; replace: string; remove: string };
}) {
  const t = useTranslations("profileEditor");
  const inputRef = useRef<HTMLInputElement>(null);
  const [local, setLocal] = useState<string | null>(null);
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  // A pasted URL that doesn't load (or an old placeholder) shows the empty
  // state rather than a black box; keyed by URL so a new pick resets it.
  const [broken, setBroken] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // The stored upload needs the authenticated path only while it isn't
  // public yet; `local` (a fresh pick) always wins over it.
  const needsAuthedPreview = !!assetId && isUploadedMedia(currentUrl) && !publicOk && !local;
  const { url: authedUrl, state: authedState } = useFileObjectUrl(
    needsAuthedPreview ? assetId : null,
  );

  useEffect(() => {
    return () => {
      if (local) URL.revokeObjectURL(local);
    };
  }, [local]);

  const candidate = local ?? (needsAuthedPreview ? authedUrl : currentUrl || null);
  const preview = candidate && candidate !== broken ? candidate : null;
  const busy = progress !== null;
  const hasSomething = !!preview || needsAuthedPreview;

  async function pick(file: File) {
    setError(null);
    const limit = kind === "image" ? MAX_IMAGE : MAX_VIDEO;
    if (file.size > limit) {
      setError(t("tooLarge", { mb: Math.round(limit / 1024 / 1024) }));
      return;
    }
    const objectUrl = URL.createObjectURL(file);
    setLocal(objectUrl);
    setProgress(0);
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    try {
      const up = await uploadWithProgress(file, setProgress, ctrl.signal);
      onUploaded(up.id);
    } catch (err) {
      setLocal(null);
      setError(err instanceof ProfileError ? err.message : t("uploadFailed"));
    } finally {
      setProgress(null);
      abortRef.current = null;
    }
  }

  function remove() {
    abortRef.current?.abort();
    setLocal(null);
    setError(null);
    onRemove();
  }

  const isImage = kind === "image";

  return (
    <div className={cn("flex flex-col gap-3", isImage ? "items-center md:w-56" : "")}>
      <div className={cn("w-full", isImage && "text-center")}>
        <div className="font-bold">{labels.title}</div>
        <p className="text-xs text-muted-foreground">{labels.hint}</p>
      </div>

      <button
        type="button"
        disabled={busy}
        onClick={() => inputRef.current?.click()}
        aria-label={hasSomething ? labels.replace : labels.choose}
        className={cn(
          "group relative overflow-hidden bg-muted ring-1 ring-border transition-shadow hover:ring-primary/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-progress",
          isImage ? "size-44 rounded-full" : "aspect-video w-full",
        )}
      >
        {preview ? (
          isImage ? (
            // Plain <img>: object URLs and cross-origin previews don't go
            // through the optimizer, and this is the owner's own editor.
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={preview}
              alt=""
              onError={() => setBroken(preview)}
              className="absolute inset-0 size-full object-cover"
            />
          ) : (
            <video
              src={preview}
              muted
              playsInline
              preload="metadata"
              onError={() => setBroken(preview)}
              className="absolute inset-0 size-full bg-black object-contain"
            />
          )
        ) : needsAuthedPreview && authedState === "loading" ? (
          <span className="absolute inset-0 animate-pulse bg-muted" />
        ) : isImage ? (
          <span className="absolute inset-0 grid place-items-center bg-gradient-to-br from-accent to-secondary font-display text-5xl text-accent-foreground">
            {initials(fallbackName) || <Camera className="size-8" />}
          </span>
        ) : (
          <span className="absolute inset-0 grid place-items-center text-muted-foreground">
            <Clapperboard className="size-10" />
          </span>
        )}

        {/* hover / progress overlay */}
        <span
          className={cn(
            "absolute inset-0 grid place-items-center text-sm font-medium text-white transition-opacity",
            busy ? "bg-black/55 opacity-100" : "bg-black/45 opacity-0 group-hover:opacity-100",
          )}
        >
          {busy ? (
            <span className="flex flex-col items-center gap-2">
              <span>{t("uploading")}</span>
              <span className="h-1.5 w-28 overflow-hidden rounded-full bg-white/30">
                <span
                  className="block h-full rounded-full bg-white transition-[width]"
                  style={{ width: `${Math.round((progress ?? 0) * 100)}%` }}
                />
              </span>
            </span>
          ) : (
            <span className="flex items-center gap-1.5">
              {isImage ? <Camera className="size-4" /> : <Clapperboard className="size-4" />}
              {hasSomething ? labels.replace : labels.choose}
            </span>
          )}
        </span>
      </button>

      <input
        ref={inputRef}
        type="file"
        accept={isImage ? "image/*" : "video/mp4,video/webm,video/quicktime"}
        className="hidden"
        onChange={(e) => {
          const f = e.target.files?.[0];
          if (f) void pick(f);
          e.target.value = "";
        }}
      />

      <div className={cn("flex items-center gap-2", isImage && "justify-center")}>
        <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => inputRef.current?.click()}>
          {hasSomething ? labels.replace : labels.choose}
        </Button>
        {(hasSomething || busy) && (
          <Button type="button" variant="ghost" size="sm" onClick={remove} aria-label={labels.remove}>
            <Trash2 /> {busy ? t("cancelUpload") : labels.remove}
          </Button>
        )}
      </div>
      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  );
}
