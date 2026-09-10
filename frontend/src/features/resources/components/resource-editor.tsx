"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  ResourceError,
  createResource,
  updateResource,
} from "@/features/resources/api";
import {
  isQuizLike,
  type Resource,
  type ResourceContent,
  type ResourceType,
  typeLabel,
} from "@/features/resources/types";
import { FileField } from "./file-field";
import { QuestionBuilder, newQuestion } from "./question-builder";

const textareaCls =
  "w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

export function ResourceEditor({
  type,
  initial,
}: {
  type: ResourceType;
  initial?: Resource;
}) {
  const router = useRouter();
  const isEdit = !!initial;

  const [title, setTitle] = useState(initial?.title ?? "");
  const [instructions, setInstructions] = useState(initial?.instructions ?? "");
  const [content, setContent] = useState<ResourceContent>(
    initial?.content ?? seedContent(type),
  );
  const [publish, setPublish] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const setC = (patch: Partial<ResourceContent>) => setContent((c) => ({ ...c, ...patch }));

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      if (isEdit) {
        await updateResource(initial.id, { title, instructions, content });
      } else {
        await createResource({ type, title, instructions, publish, content });
      }
      router.push("/resources");
      router.refresh();
    } catch (err) {
      setError(err instanceof ResourceError ? err.message : "Something went wrong.");
      setSaving(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-6">
      <div>
        <Link
          href="/resources"
          className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeft className="size-4" />
          Resources
        </Link>
        <h1 className="mt-2 font-display text-2xl">
          {isEdit ? "Edit" : "New"} {typeLabel(type).toLowerCase()}
        </h1>
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="title">Title</Label>
        <Input id="title" required value={title} onChange={(e) => setTitle(e.target.value)} />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor="instructions">Instructions for the student (optional)</Label>
        <textarea
          id="instructions"
          rows={2}
          className={textareaCls}
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
        />
      </div>

      {/* --- type-specific --- */}

      {type === "material" && (
        <>
          <FileField
            label="File"
            value={content.fileAssetId}
            onChange={(id) => setC({ fileAssetId: id, url: id ? "" : content.url })}
          />
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="url">…or a link</Label>
            <Input
              id="url"
              placeholder="https://…"
              value={content.url ?? ""}
              onChange={(e) => setC({ url: e.target.value })}
              disabled={!!content.fileAssetId}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="desc">Description (optional)</Label>
            <textarea
              id="desc"
              rows={2}
              className={textareaCls}
              value={content.description ?? ""}
              onChange={(e) => setC({ description: e.target.value })}
            />
          </div>
        </>
      )}

      {type === "article" && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="body">Body</Label>
          <textarea
            id="body"
            rows={12}
            className={textareaCls}
            placeholder="Markdown is supported."
            value={content.body ?? ""}
            onChange={(e) => setC({ body: e.target.value })}
          />
        </div>
      )}

      {type === "writing" && (
        <>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="prompt">Prompt</Label>
            <textarea
              id="prompt"
              rows={4}
              className={textareaCls}
              value={content.prompt ?? ""}
              onChange={(e) => setC({ prompt: e.target.value })}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="minwords">Minimum words (optional)</Label>
            <Input
              id="minwords"
              type="number"
              min={0}
              className="w-32"
              value={content.minWords ?? ""}
              onChange={(e) => setC({ minWords: Number(e.target.value) || 0 })}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="rubric">Rubric / notes for grading (optional)</Label>
            <textarea
              id="rubric"
              rows={3}
              className={textareaCls}
              value={content.rubric ?? ""}
              onChange={(e) => setC({ rubric: e.target.value })}
            />
          </div>
        </>
      )}

      {type === "listening" && (
        <FileField
          label="Audio"
          accept="audio/*"
          value={content.audioAssetId}
          onChange={(id) => setC({ audioAssetId: id })}
        />
      )}

      {type === "reading" && (
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="passage">Passage</Label>
          <textarea
            id="passage"
            rows={8}
            className={textareaCls}
            value={content.passage ?? ""}
            onChange={(e) => setC({ passage: e.target.value })}
          />
        </div>
      )}

      {isQuizLike(type) && (
        <div className="flex flex-col gap-2">
          <span className="text-sm font-medium">Questions</span>
          <QuestionBuilder
            questions={content.questions ?? []}
            onChange={(questions) => setC({ questions })}
          />
        </div>
      )}

      {error && (
        <p role="alert" className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {error}
        </p>
      )}

      <div className="sticky bottom-0 -mx-4 flex items-center gap-3 border-t border-border bg-background px-4 py-4 sm:-mx-6 sm:px-6">
        <Button type="submit" disabled={saving}>
          {saving ? "Saving…" : isEdit ? "Save changes" : "Create"}
        </Button>
        {!isEdit && (
          <label className="flex items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={publish}
              onChange={(e) => setPublish(e.target.checked)}
              className="accent-primary"
            />
            Publish now
          </label>
        )}
      </div>
    </form>
  );
}

function seedContent(type: ResourceType): ResourceContent {
  if (isQuizLike(type)) return { questions: [newQuestion()] };
  return {};
}
