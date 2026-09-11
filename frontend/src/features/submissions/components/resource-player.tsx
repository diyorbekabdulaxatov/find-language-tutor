"use client";

import { useEffect, useRef, useState } from "react";
import { Download, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { fetchFileObjectUrl } from "@/features/resources/api";
import { hasSubmissionFlow, type AttachedResource, type Submission } from "@/features/resources/types";
import {
  SubmissionError,
  saveSubmissionAnswers,
  startSubmission,
  submitSubmission,
} from "@/features/submissions/api";
import { SubmissionStatusBadge } from "./submission-status-badge";

const AUTOSAVE_MS = 800;

/**
 * Renders one attached resource for the student, dispatched on `attachment.type`.
 * `material` / `article` are static (no submission). `quiz` / `listening` /
 * `reading` / `writing` start (or resume — idempotent) a submission on mount,
 * autosave answers, and submit for grading.
 */
export function ResourcePlayer({
  attachment,
  bookingId,
  onSubmissionChange,
}: {
  attachment: AttachedResource;
  bookingId: string;
  onSubmissionChange?: (submission: Submission) => void;
}) {
  const needsSubmission = hasSubmissionFlow(attachment.type);

  const [submission, setSubmission] = useState<Submission | null>(null);
  const [answers, setAnswers] = useState<Record<string, string[]>>({});
  const [state, setState] = useState<"loading" | "ready" | "error">(
    needsSubmission ? "loading" : "ready",
  );
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Latest callback in a ref so the mount effect below doesn't need it as a
  // dependency (the caller typically passes a fresh arrow function each render).
  const onSubmissionChangeRef = useRef(onSubmissionChange);
  useEffect(() => {
    onSubmissionChangeRef.current = onSubmissionChange;
  });

  useEffect(() => {
    if (!needsSubmission) return;
    let alive = true;
    async function load() {
      try {
        const s = await startSubmission({
          resourceId: attachment.resourceId,
          bookingId,
        });
        if (!alive) return;
        setSubmission(s);
        setAnswers(s.answers);
        setState("ready");
        onSubmissionChangeRef.current?.(s);
      } catch (err) {
        if (!alive) return;
        setError(
          err instanceof SubmissionError ? err.message : "Could not load this homework.",
        );
        setState("error");
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [attachment.resourceId, bookingId, needsSubmission]);

  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
  }, []);

  const locked = !submission || submission.status !== "in_progress";

  function updateAnswers(next: Record<string, string[]>) {
    setAnswers(next);
    if (!submission || submission.status !== "in_progress") return;
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      void saveSubmissionAnswers(submission.id, next).catch(() => {
        /* best-effort autosave; the next save or the final submit will retry */
      });
    }, AUTOSAVE_MS);
  }

  async function handleSubmit() {
    if (!submission) return;
    setSubmitting(true);
    setError(null);
    try {
      if (saveTimer.current) clearTimeout(saveTimer.current);
      await saveSubmissionAnswers(submission.id, answers).catch(() => {
        /* the submit call below still carries the server's last-saved copy */
      });
      const result = await submitSubmission(submission.id);
      setSubmission(result);
      setAnswers(result.answers);
      onSubmissionChange?.(result);
    } catch (err) {
      setError(err instanceof SubmissionError ? err.message : "Could not submit that homework.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {attachment.instructions && (
        <p className="text-sm text-muted-foreground">{attachment.instructions}</p>
      )}

      {attachment.type === "material" && <MaterialView attachment={attachment} />}
      {attachment.type === "article" && <ArticleView attachment={attachment} />}

      {needsSubmission && (
        <>
          {state === "loading" && (
            <div className="h-40 animate-pulse rounded-xl bg-muted" />
          )}
          {state === "error" && (
            <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </p>
          )}
          {state === "ready" && submission && (
            <>
              <div className="flex items-center justify-between gap-2">
                <SubmissionStatusBadge
                  status={submission.status}
                  autoScore={submission.autoScore}
                  autoMax={submission.autoMax}
                  teacherScore={submission.teacherScore}
                />
                {submission.status === "graded" && submission.autoScore == null && (
                  <span className="text-xs text-muted-foreground">
                    {submission.teacherFeedback ? "See feedback below." : "Graded, no written feedback."}
                  </span>
                )}
              </div>

              {(attachment.type === "quiz" ||
                attachment.type === "listening" ||
                attachment.type === "reading") && (
                <QuizForm
                  attachment={attachment}
                  answers={answers}
                  locked={locked}
                  onChange={updateAnswers}
                />
              )}

              {attachment.type === "writing" && (
                <WritingForm
                  attachment={attachment}
                  answers={answers}
                  locked={locked}
                  onChange={updateAnswers}
                />
              )}

              {submission.status === "graded" && submission.teacherFeedback && (
                <div className="rounded-xl border border-border bg-muted/40 p-3 text-sm">
                  <p className="font-medium">Teacher feedback</p>
                  <p className="mt-1 whitespace-pre-wrap text-muted-foreground">
                    {submission.teacherFeedback}
                  </p>
                </div>
              )}

              {error && (
                <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </p>
              )}

              {!locked && (
                <Button onClick={() => void handleSubmit()} disabled={submitting} className="self-start">
                  {submitting ? "Submitting…" : "Submit"}
                </Button>
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}

/**
 * Loads a stored file's bytes through the authenticated endpoint into an
 * object URL, since GET /v1/files/{id} needs a bearer token no plain
 * `<a href>` / `<audio src>` can attach. Revokes the URL on unmount / id change.
 */
function useFileObjectUrl(fileAssetId: string | undefined) {
  const [url, setUrl] = useState<string | null>(null);
  // Lazy initializer, not an effect — "no id yet" is derivable at first
  // render, so it never needs a synchronous setState inside the effect body.
  const [state, setState] = useState<"idle" | "loading" | "ready" | "error">(() =>
    fileAssetId ? "loading" : "idle",
  );

  useEffect(() => {
    if (!fileAssetId) return;
    let alive = true;
    let objectUrl: string | null = null;
    fetchFileObjectUrl(fileAssetId)
      .then((u) => {
        if (!alive) {
          URL.revokeObjectURL(u);
          return;
        }
        objectUrl = u;
        setUrl(u);
        setState("ready");
      })
      .catch(() => {
        if (alive) setState("error");
      });
    return () => {
      alive = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [fileAssetId]);

  return { url, state };
}

function MaterialView({ attachment }: { attachment: AttachedResource }) {
  const { content } = attachment;
  const { url: fileObjectUrl, state: fileState } = useFileObjectUrl(content.fileAssetId);
  return (
    <div className="flex flex-col gap-3">
      {content.description && (
        <p className="whitespace-pre-wrap text-sm">{content.description}</p>
      )}
      {content.fileAssetId && fileState === "error" && (
        <p className="text-sm text-destructive">Could not load the attached file.</p>
      )}
      {content.fileAssetId && fileState !== "error" && (
        <Button asChild variant="outline" className="self-start" disabled={!fileObjectUrl}>
          <a href={fileObjectUrl ?? undefined} download target="_blank" rel="noreferrer">
            <Download className="size-4" />
            {fileObjectUrl ? "Download file" : "Loading file…"}
          </a>
        </Button>
      )}
      {content.url && (
        <Button asChild variant="outline" className="self-start">
          <a href={content.url} target="_blank" rel="noreferrer">
            <ExternalLink className="size-4" />
            Open link
          </a>
        </Button>
      )}
      {!content.description && !content.fileAssetId && !content.url && (
        <p className="text-sm text-muted-foreground">Nothing attached to this material yet.</p>
      )}
    </div>
  );
}

function AudioPlayer({ fileAssetId }: { fileAssetId: string }) {
  const { url, state } = useFileObjectUrl(fileAssetId);
  if (state === "error") {
    return <p className="text-sm text-destructive">Could not load the audio.</p>;
  }
  if (!url) {
    return <div className="h-10 animate-pulse rounded-lg bg-muted" />;
  }
  return (
    <audio controls className="w-full" src={url}>
      <track kind="captions" />
    </audio>
  );
}

function ArticleView({ attachment }: { attachment: AttachedResource }) {
  return (
    <div className="rounded-xl border border-border bg-muted/30 p-4 text-sm whitespace-pre-wrap">
      {attachment.content.body || "This article has no content yet."}
    </div>
  );
}

function QuizForm({
  attachment,
  answers,
  locked,
  onChange,
}: {
  attachment: AttachedResource;
  answers: Record<string, string[]>;
  locked: boolean;
  onChange: (next: Record<string, string[]>) => void;
}) {
  const { content } = attachment;
  return (
    <div className="flex flex-col gap-4">
      {attachment.type === "listening" && content.audioAssetId && (
        <AudioPlayer fileAssetId={content.audioAssetId} />
      )}
      {attachment.type === "reading" && content.passage && (
        <div className="rounded-xl border border-border bg-muted/30 p-4 text-sm whitespace-pre-wrap">
          {content.passage}
        </div>
      )}
      {(content.questions ?? []).map((q, i) => {
        const value = answers[q.id] ?? [];
        return (
          <div key={q.id} className="rounded-xl border border-border bg-card p-4">
            <p className="text-sm font-medium">
              {i + 1}. {q.prompt}
            </p>
            {q.kind === "single" && (
              <div className="mt-2 flex flex-col gap-1.5">
                {q.choices.map((c) => (
                  <label key={c.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="radio"
                      name={q.id}
                      className="accent-primary"
                      checked={value.includes(c.id)}
                      disabled={locked}
                      onChange={() => onChange({ ...answers, [q.id]: [c.id] })}
                    />
                    {c.text}
                  </label>
                ))}
              </div>
            )}
            {q.kind === "multi" && (
              <div className="mt-2 flex flex-col gap-1.5">
                {q.choices.map((c) => (
                  <label key={c.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      className="accent-primary"
                      checked={value.includes(c.id)}
                      disabled={locked}
                      onChange={() =>
                        onChange({
                          ...answers,
                          [q.id]: value.includes(c.id)
                            ? value.filter((id) => id !== c.id)
                            : [...value, c.id],
                        })
                      }
                    />
                    {c.text}
                  </label>
                ))}
              </div>
            )}
            {q.kind === "text" && (
              <input
                type="text"
                className="mt-2 w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50"
                value={value[0] ?? ""}
                disabled={locked}
                onChange={(e) => onChange({ ...answers, [q.id]: [e.target.value] })}
                placeholder="Your answer"
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function WritingForm({
  attachment,
  answers,
  locked,
  onChange,
}: {
  attachment: AttachedResource;
  answers: Record<string, string[]>;
  locked: boolean;
  onChange: (next: Record<string, string[]>) => void;
}) {
  const text = answers.text?.[0] ?? "";
  const wordCount = text.trim() === "" ? 0 : text.trim().split(/\s+/).length;
  const minWords = attachment.content.minWords;

  return (
    <div className="flex flex-col gap-2">
      {attachment.content.prompt && (
        <p className="text-sm whitespace-pre-wrap">{attachment.content.prompt}</p>
      )}
      <textarea
        rows={10}
        value={text}
        disabled={locked}
        onChange={(e) => onChange({ ...answers, text: [e.target.value] })}
        placeholder="Write your response here…"
        className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50"
      />
      <p className="text-xs text-muted-foreground">
        {wordCount} word{wordCount === 1 ? "" : "s"}
        {minWords ? ` · recommended at least ${minWords}` : ""}
      </p>
    </div>
  );
}
