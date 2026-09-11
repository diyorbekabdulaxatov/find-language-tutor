"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, ImageIcon, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { formatMoney } from "@/lib/format";
import { ResourceError, uploadFile } from "@/features/resources/api";
import {
  CourseError,
  createCourse,
  deleteCourse,
  setCourseArchived,
  setCoursePublished,
  updateCourse,
} from "@/features/courses/api";
import type { CourseDetail } from "@/features/courses/types";
import type { Money } from "@/types/teacher";
import { useFileObjectUrl } from "../use-file-object-url";
import { CurriculumBuilder } from "./curriculum-builder";

const textareaCls =
  "w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

/** amount_minor -> a plain number of whole units (so'm / dollars) for the price input. */
function unitsFromMinor(amountMinor: number): number {
  return amountMinor / 100;
}

/**
 * Course editor. With no `initial`, a minimal create form (title/subtitle/
 * description/price) that creates an empty draft and hands off to the full
 * editor. With `initial`, the full editor: course fields + lifecycle
 * controls + the curriculum builder — all curriculum/lifecycle mutations
 * return the full CourseDetail, so this component keeps it in local state
 * and never needs to refetch.
 */
export function CourseEditor({ initial }: { initial?: CourseDetail }) {
  const router = useRouter();
  const isEdit = !!initial;

  const [course, setCourse] = useState<CourseDetail | undefined>(initial);

  const [title, setTitle] = useState(initial?.title ?? "");
  const [subtitle, setSubtitle] = useState(initial?.subtitle ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [priceUnits, setPriceUnits] = useState(
    initial ? unitsFromMinor(initial.price.amountMinor) : 0,
  );
  const [currency, setCurrency] = useState<Money["currency"]>(initial?.price.currency ?? "UZS");
  const [coverAssetId, setCoverAssetId] = useState<string | null>(initial?.coverAssetId ?? null);
  const [coverBusy, setCoverBusy] = useState(false);
  const coverInputRef = useRef<HTMLInputElement>(null);

  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [busy, setBusy] = useState(false);

  const { url: coverUrl, state: coverState } = useFileObjectUrl(coverAssetId);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      const priceAmountMinor = Math.round(priceUnits * 100);
      if (isEdit && course) {
        const updated = await updateCourse(course.id, {
          title,
          subtitle,
          description,
          coverAssetId,
          priceAmountMinor,
          priceCurrency: currency,
        });
        setCourse(updated);
      } else {
        const created = await createCourse({
          title,
          subtitle,
          description,
          priceAmountMinor,
          priceCurrency: currency,
        });
        router.push(`/courses/${created.id}/edit`);
        return;
      }
    } catch (err) {
      setError(err instanceof CourseError ? err.message : "Something went wrong.");
    } finally {
      setSaving(false);
    }
  }

  async function pickCover(file: File) {
    setError(null);
    setCoverBusy(true);
    try {
      const up = await uploadFile(file);
      setCoverAssetId(up.id);
    } catch (err) {
      setError(err instanceof ResourceError ? err.message : "Could not upload the cover image.");
    }
    setCoverBusy(false);
  }

  async function run(fn: () => Promise<CourseDetail>) {
    setBusy(true);
    setError(null);
    try {
      const updated = await fn();
      setCourse(updated);
    } catch (err) {
      setError(err instanceof CourseError ? err.message : "Something went wrong.");
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    if (!course) return;
    if (!confirm(`Delete "${course.title}"? This can't be undone.`)) return;
    setBusy(true);
    setError(null);
    try {
      await deleteCourse(course.id);
      router.push("/courses");
      router.refresh();
    } catch (err) {
      setError(err instanceof CourseError ? err.message : "Could not delete the course.");
      setBusy(false);
    }
  }

  // The wire doesn't expose the backend's one-way "ever published" latch —
  // only the current, reversible `status`. A course that's currently
  // published is unambiguously blocked; one that's back in draft after a
  // prior publish looks identical to one that was never published, so that
  // case is only caught server-side (409 course_in_use), surfaced via `error`.
  const knownBlocked = course?.status === "published";

  return (
    <div className="flex flex-col gap-6">
      <div>
        <Link
          href="/courses"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          My courses
        </Link>
        <h1 className="mt-2 font-display text-2xl">{isEdit ? "Edit course" : "New course"}</h1>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="title">Title</Label>
          <Input id="title" required value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="subtitle">Subtitle (optional)</Label>
          <Input id="subtitle" value={subtitle} onChange={(e) => setSubtitle(e.target.value)} />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="description">Description (optional)</Label>
          <textarea
            id="description"
            rows={4}
            className={textareaCls}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        <div className="flex flex-wrap items-end gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="price">Price</Label>
            <Input
              id="price"
              type="number"
              min={0}
              step="0.01"
              className="w-32"
              value={priceUnits}
              onChange={(e) => setPriceUnits(Math.max(0, Number(e.target.value) || 0))}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="currency">Currency</Label>
            <Select value={currency} onValueChange={(v) => setCurrency(v as Money["currency"])}>
              <SelectTrigger id="currency" className="w-24">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="UZS">UZS</SelectItem>
                <SelectItem value="USD">USD</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <p className="pb-1.5 text-sm text-muted-foreground">
            {priceUnits === 0
              ? "Free course"
              : formatMoney({ amountMinor: Math.round(priceUnits * 100), currency })}
          </p>
        </div>

        {isEdit && (
          <div className="flex flex-col gap-1.5">
            <span className="text-sm font-medium">Cover image</span>
            {coverAssetId ? (
              <div className="flex items-start gap-3">
                <div className="relative aspect-video w-48 shrink-0 overflow-hidden rounded-lg border border-border bg-muted">
                  {coverState === "ready" && coverUrl ? (
                    // eslint-disable-next-line @next/next/no-img-element -- object URL, not a static asset
                    <img src={coverUrl} alt="" className="size-full object-cover" />
                  ) : coverState === "error" ? (
                    <p className="p-2 text-xs text-destructive">Could not load the cover.</p>
                  ) : (
                    <div className="size-full animate-pulse" />
                  )}
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={coverBusy}
                  onClick={() => setCoverAssetId(null)}
                >
                  <X className="size-4" />
                  Remove
                </Button>
              </div>
            ) : (
              <div>
                <input
                  ref={coverInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void pickCover(f);
                    e.target.value = "";
                  }}
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={coverBusy}
                  onClick={() => coverInputRef.current?.click()}
                >
                  <ImageIcon className="size-4" />
                  {coverBusy ? "Uploading…" : "Choose a cover image"}
                </Button>
              </div>
            )}
          </div>
        )}

        {error && (
          <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}

        <div className="sticky bottom-0 -mx-4 flex flex-wrap items-center gap-3 border-t border-border bg-background px-4 py-4 sm:-mx-6 sm:px-6">
          <Button type="submit" disabled={saving}>
            {saving ? "Saving…" : isEdit ? "Save changes" : "Create course"}
          </Button>

          {isEdit && course && (
            <>
              {course.status === "published" ? (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => run(() => setCoursePublished(course.id, false))}
                >
                  Unpublish
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => run(() => setCoursePublished(course.id, true))}
                >
                  Publish
                </Button>
              )}
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => run(() => setCourseArchived(course.id, !course.archived))}
              >
                {course.archived ? "Restore" : "Archive"}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="ml-auto text-destructive hover:text-destructive"
                disabled={busy || knownBlocked}
                title={knownBlocked ? "Published courses can't be deleted — archive it instead." : undefined}
                onClick={() => void handleDelete()}
              >
                Delete
              </Button>
            </>
          )}
        </div>
        {isEdit && knownBlocked && (
          <p className="-mt-3 text-xs text-muted-foreground">
            This course has been published, so it can&apos;t be deleted — archive it instead.
          </p>
        )}
      </form>

      {isEdit && course && (
        <CurriculumBuilder course={course} onCourseChange={setCourse} />
      )}
    </div>
  );
}
