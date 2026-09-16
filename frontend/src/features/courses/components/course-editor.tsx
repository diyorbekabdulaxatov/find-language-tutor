"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, ImageIcon, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
  const t = useTranslations("courses");
  const locale = useLocale();

  const [course, setCourse] = useState<CourseDetail | undefined>(initial);

  const [title, setTitle] = useState(initial?.title ?? "");
  const [subtitle, setSubtitle] = useState(initial?.subtitle ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [priceUnits, setPriceUnits] = useState(
    initial ? unitsFromMinor(initial.price.amountMinor) : 0,
  );
  // Courses are priced in so'm like lessons; the wire field is fixed to UZS.
  const currency = "UZS" as const;
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
      setError(err instanceof CourseError ? err.message : t("somethingWrong"));
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
      setError(err instanceof ResourceError ? err.message : t("couldNotUploadCover"));
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
      setError(err instanceof CourseError ? err.message : t("somethingWrong"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDelete() {
    if (!course) return;
    if (!confirm(t("confirmDelete", { title: course.title }))) return;
    setBusy(true);
    setError(null);
    try {
      await deleteCourse(course.id);
      router.push("/courses");
      router.refresh();
    } catch (err) {
      setError(err instanceof CourseError ? err.message : t("couldNotDelete"));
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
          {t("libraryTitle")}
        </Link>
        <h1 className="mt-2 font-display text-2xl">{isEdit ? t("metaEdit") : t("metaNew")}</h1>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="title">{t("title")}</Label>
          <Input id="title" required maxLength={120} value={title} onChange={(e) => setTitle(e.target.value)} />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="subtitle">{t("subtitle")}</Label>
          <Input id="subtitle" maxLength={200} value={subtitle} onChange={(e) => setSubtitle(e.target.value)} />
        </div>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="description">{t("description")}</Label>
          <textarea
            id="description"
            rows={4}
            maxLength={8000}
            className={textareaCls}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </div>

        <div className="flex flex-wrap items-end gap-3">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="price">{t("price")}</Label>
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
            <Label htmlFor="currency">{t("currency")}</Label>
            <Input id="currency" value={currency} readOnly disabled className="w-24" />
          </div>
          <p className="pb-1.5 text-sm text-muted-foreground">
            {priceUnits === 0
              ? t("freeCourse")
              : formatMoney({ amountMinor: Math.round(priceUnits * 100), currency }, locale)}
          </p>
        </div>

        {isEdit && (
          <div className="flex flex-col gap-1.5">
            <span className="text-sm font-medium">{t("coverImage")}</span>
            {coverAssetId ? (
              <div className="flex items-start gap-3">
                <div className="relative aspect-video w-48 shrink-0 overflow-hidden rounded-lg border border-border bg-muted">
                  {coverState === "ready" && coverUrl ? (
                    // eslint-disable-next-line @next/next/no-img-element -- object URL, not a static asset
                    <img src={coverUrl} alt="" className="size-full object-cover" />
                  ) : coverState === "error" ? (
                    <p className="p-2 text-xs text-destructive">{t("couldNotLoadCover")}</p>
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
                  {t("remove")}
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
                  {coverBusy ? t("uploading") : t("chooseCover")}
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
            {saving ? t("saving") : isEdit ? t("saveChanges") : t("createCourse")}
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
                  {t("unpublish")}
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  disabled={busy}
                  onClick={() => run(() => setCoursePublished(course.id, true))}
                >
                  {t("publish")}
                </Button>
              )}
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => run(() => setCourseArchived(course.id, !course.archived))}
              >
                {course.archived ? t("restore") : t("archive")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="ml-auto text-destructive hover:text-destructive"
                disabled={busy || knownBlocked}
                title={knownBlocked ? t("publishedNoDelete") : undefined}
                onClick={() => void handleDelete()}
              >
                {t("delete")}
              </Button>
            </>
          )}
        </div>
        {isEdit && knownBlocked && (
          <p className="-mt-3 text-xs text-muted-foreground">
            {t("publishedNoDeleteLong")}
          </p>
        )}
      </form>

      {isEdit && course && (
        <CurriculumBuilder course={course} onCourseChange={setCourse} />
      )}
    </div>
  );
}
