"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ChevronDown, ChevronUp, PenLine } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { getResource } from "@/features/resources/api";
import type { Submission } from "@/features/resources/types";
import {
  SubmissionError,
  gradeSubmission,
  listSubmissionInbox,
  type InboxStatus,
} from "@/features/submissions/api";

const FILTERS: { value: InboxStatus; label: string }[] = [
  { value: "submitted", label: "Awaiting grade" },
  { value: "graded", label: "Graded" },
  { value: "all", label: "All" },
];

function fmtDate(iso: string | null): string {
  if (!iso) return "—";
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

/**
 * The teacher's grading inbox — `writing` submissions awaiting a grade by
 * default. Each row expands into the essay text plus a score/feedback form.
 */
export function GradingInbox() {
  const [filter, setFilter] = useState<InboxStatus>("submitted");
  const [items, setItems] = useState<Submission[]>([]);
  const [titles, setTitles] = useState<Record<string, string>>({});
  const [state, setState] = useState<"loading" | "ready" | "error" | "no-teacher">("loading");
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    async function load() {
      setState("loading");
      try {
        const { submissions } = await listSubmissionInbox({ status: filter });
        if (!alive) return;
        setItems(submissions);
        setState("ready");
      } catch (err) {
        if (!alive) return;
        setState(
          err instanceof SubmissionError && err.code === "no_teacher_profile"
            ? "no-teacher"
            : "error",
        );
      }
    }
    void load();
    return () => {
      alive = false;
    };
  }, [filter]);

  // Best-effort resource titles (the inbox rows only carry ids on the wire).
  useEffect(() => {
    const missing = [...new Set(items.map((s) => s.resourceId))].filter((id) => !(id in titles));
    if (missing.length === 0) return;
    let alive = true;
    async function loadTitles() {
      const entries = await Promise.all(
        missing.map(async (id) => {
          try {
            const r = await getResource(id);
            return [id, r.title] as const;
          } catch {
            return [id, "Homework"] as const;
          }
        }),
      );
      if (!alive) return;
      setTitles((prev) => ({ ...prev, ...Object.fromEntries(entries) }));
    }
    void loadTitles();
    return () => {
      alive = false;
    };
    // `titles` intentionally excluded — this effect only adds newly-seen ids.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [items]);

  function onGraded(id: string, updated: Submission) {
    setItems((prev) => {
      if (filter === "submitted") return prev.filter((s) => s.id !== id);
      return prev.map((s) => (s.id === id ? updated : s));
    });
  }

  if (state === "no-teacher") {
    return (
      <div className="rounded-2xl border border-border bg-card px-6 py-12 text-center">
        <p className="text-sm text-muted-foreground">
          Grading is part of your teaching toolkit — create a teacher profile first.
        </p>
        <Button asChild className="mt-4">
          <Link href="/dashboard">Go to the dashboard</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-3xl">Homework to grade</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Writing tasks your students have submitted.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => setFilter(f.value)}
            className={cn(
              "rounded-lg border px-2.5 py-1 text-xs font-medium transition-colors",
              filter === f.value ? "border-primary bg-accent" : "border-border hover:bg-muted",
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {state === "error" ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
          Could not load your grading inbox.
        </p>
      ) : state === "loading" ? (
        <div className="h-48 animate-pulse rounded-2xl bg-muted" />
      ) : items.length === 0 ? (
        <p className="rounded-2xl border border-border bg-card px-4 py-12 text-center text-sm text-muted-foreground">
          Nothing here.
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((s) => (
            <GradingRow
              key={s.id}
              submission={s}
              title={titles[s.resourceId] ?? "Homework"}
              expanded={expanded === s.id}
              onToggle={() => setExpanded((cur) => (cur === s.id ? null : s.id))}
              onGraded={(updated) => onGraded(s.id, updated)}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function GradingRow({
  submission: s,
  title,
  expanded,
  onToggle,
  onGraded,
}: {
  submission: Submission;
  title: string;
  expanded: boolean;
  onToggle: () => void;
  onGraded: (updated: Submission) => void;
}) {
  const [score, setScore] = useState(s.teacherScore != null ? String(s.teacherScore) : "");
  const [feedback, setFeedback] = useState(s.teacherFeedback);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const essay = s.answers.text?.[0] ?? "";
  const wordCount = essay.trim() === "" ? 0 : essay.trim().split(/\s+/).length;

  async function submit() {
    setBusy(true);
    setError(null);
    try {
      const trimmed = score.trim();
      const parsed = trimmed === "" ? null : Number(trimmed);
      if (parsed !== null && (!Number.isFinite(parsed) || parsed < 0)) {
        setError("Score must be a non-negative number, or left blank.");
        setBusy(false);
        return;
      }
      const updated = await gradeSubmission(s.id, { score: parsed, feedback: feedback.trim() });
      onGraded(updated);
    } catch (err) {
      setError(err instanceof SubmissionError ? err.message : "Could not save the grade.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="rounded-2xl border border-border bg-card p-4">
      <button
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-3 text-left"
      >
        <div className="flex items-center gap-3">
          <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground">
            <PenLine className="size-4" />
          </span>
          <div>
            <p className="text-sm font-medium">{title}</p>
            <p className="text-xs text-muted-foreground">
              {s.status === "graded" ? "Graded" : "Submitted"} {fmtDate(s.submittedAt ?? s.gradedAt)}
              {" · "}
              <Link
                href={`/bookings/${s.bookingId}`}
                className="underline hover:text-foreground"
                onClick={(e) => e.stopPropagation()}
              >
                View lesson
              </Link>
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {s.status === "graded" && (
            <Badge className="bg-primary/15 text-primary">
              {s.teacherScore != null ? `Score ${s.teacherScore}` : "Graded"}
            </Badge>
          )}
          {expanded ? (
            <ChevronUp className="size-4 text-muted-foreground" />
          ) : (
            <ChevronDown className="size-4 text-muted-foreground" />
          )}
        </div>
      </button>

      {expanded && (
        <div className="mt-4 flex flex-col gap-3 border-t border-border pt-4">
          <div>
            <p className="text-xs font-medium text-muted-foreground">
              Essay ({wordCount} word{wordCount === 1 ? "" : "s"})
            </p>
            <div className="mt-1 max-h-72 overflow-y-auto rounded-xl border border-border bg-background/40 p-3 text-sm whitespace-pre-wrap">
              {essay || "The student hasn't written anything yet."}
            </div>
          </div>

          {s.status === "graded" ? (
            // The API only grades a `submitted` submission once — there is no
            // re-grade endpoint, so a graded row is a read-only record.
            <div className="rounded-xl border border-border bg-muted/30 p-3 text-sm">
              <p className="font-medium">
                {s.teacherScore != null ? `Score: ${s.teacherScore}` : "Graded, no score"}
              </p>
              {s.teacherFeedback && (
                <p className="mt-1 whitespace-pre-wrap text-muted-foreground">
                  {s.teacherFeedback}
                </p>
              )}
            </div>
          ) : (
            <>
              <div className="flex flex-col gap-1.5">
                <label htmlFor={`score-${s.id}`} className="text-sm font-medium">
                  Score (optional)
                </label>
                <input
                  id={`score-${s.id}`}
                  type="number"
                  min={0}
                  value={score}
                  onChange={(e) => setScore(e.target.value)}
                  placeholder="No score — feedback only"
                  className="w-32 rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <label htmlFor={`feedback-${s.id}`} className="text-sm font-medium">
                  Feedback
                </label>
                <textarea
                  id={`feedback-${s.id}`}
                  rows={4}
                  value={feedback}
                  onChange={(e) => setFeedback(e.target.value)}
                  placeholder="What did they do well? What should they work on?"
                  className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                />
              </div>

              {error && (
                <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </p>
              )}

              <Button onClick={() => void submit()} disabled={busy} className="self-start">
                {busy ? "Saving…" : "Save grade"}
              </Button>
            </>
          )}
        </div>
      )}
    </li>
  );
}
