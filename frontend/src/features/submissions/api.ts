/**
 * Submissions data-access (browser) — starting, saving, submitting and
 * grading homework. Wraps `browserApi`, maps wire ↔ view-model (camelCase),
 * and throws a typed SubmissionError on failure. Mirrors the conventions in
 * `@/features/resources/api`.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type { Submission } from "@/features/resources/types";

export class SubmissionError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "SubmissionError";
    this.code = code;
    this.status = status;
  }
}

type ErrBody = { error?: { code?: string; message?: string } };
function toErr(e: unknown, status: number, fallback: string): SubmissionError {
  const b = e as ErrBody | undefined;
  return new SubmissionError(b?.error?.message ?? fallback, b?.error?.code ?? "unknown", status);
}

type WireSubmission = components["schemas"]["Submission"];

function toSubmission(s: WireSubmission): Submission {
  return {
    id: s.id,
    resourceId: s.resource_id,
    bookingId: s.booking_id,
    status: s.status,
    answers: s.answers ?? {},
    autoScore: s.auto_score,
    autoMax: s.auto_max,
    teacherScore: s.teacher_score,
    teacherFeedback: s.teacher_feedback,
    submittedAt: s.submitted_at,
    gradedAt: s.graded_at,
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  };
}

/** Starts (or resumes — idempotent) the caller's submission for a homework
 *  attachment. The caller must be the booking's student. */
export async function startSubmission(input: {
  resourceId: string;
  bookingId: string;
}): Promise<Submission> {
  const { data, error, response } = await browserApi.POST("/v1/submissions", {
    body: { resource_id: input.resourceId, booking_id: input.bookingId },
  });
  if (error || !data) throw toErr(error, response.status, "Could not start that homework.");
  return toSubmission(data);
}

/** Saves draft answers. Owner-only, and only while `in_progress`. */
export async function saveSubmissionAnswers(
  id: string,
  answers: Record<string, string[]>,
): Promise<Submission> {
  const { data, error, response } = await browserApi.PATCH("/v1/submissions/{id}", {
    params: { path: { id } },
    body: { answers },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save your answers.");
  return toSubmission(data);
}

export async function getSubmission(id: string): Promise<Submission> {
  const { data, error, response } = await browserApi.GET("/v1/submissions/{id}", {
    params: { path: { id } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load that submission.");
  return toSubmission(data);
}

/** Submits the homework. Quiz-like resources auto-grade immediately; writing
 *  lands on `submitted` and awaits a teacher's grade. */
export async function submitSubmission(id: string): Promise<Submission> {
  const { data, error, response } = await browserApi.POST("/v1/submissions/{id}/submit", {
    params: { path: { id } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not submit that homework.");
  return toSubmission(data);
}

/** Grades a `writing` submission. `score` is nullable — feedback-only grading
 *  is valid. Teacher-owner of the booking only. */
export async function gradeSubmission(
  id: string,
  input: { score: number | null; feedback: string },
): Promise<Submission> {
  const { data, error, response } = await browserApi.POST("/v1/submissions/{id}/grade", {
    params: { path: { id } },
    body: { score: input.score, feedback: input.feedback },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save the grade.");
  return toSubmission(data);
}

export type InboxStatus = "in_progress" | "submitted" | "graded" | "all";

/** The teacher's grading inbox — their own resources' submissions. Defaults
 *  to `submitted` (writing tasks awaiting a grade). */
export async function listSubmissionInbox(opts: {
  status?: InboxStatus;
  page?: number;
}): Promise<{ submissions: Submission[]; total: number }> {
  const { data, error, response } = await browserApi.GET("/v1/submissions", {
    params: { query: { status: opts.status, page: opts.page } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load your grading inbox.");
  return { submissions: data.submissions.map(toSubmission), total: data.total };
}
