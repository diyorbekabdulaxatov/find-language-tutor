/** View-models for the resource library. camelCase; the wire shapes live in
 *  the generated schema. */

export type ResourceType =
  | "material"
  | "article"
  | "quiz"
  | "listening"
  | "reading"
  | "writing";

export type ResourceStatus = "draft" | "published";

export type QuestionKind = "single" | "multi" | "text";

export interface Choice {
  id: string;
  text: string;
}

export interface Question {
  id: string;
  prompt: string;
  kind: QuestionKind;
  choices: Choice[];
  /** choice ids (single/multi) or accepted answer strings (text) */
  correct: string[];
  points: number;
}

export interface ResourceContent {
  // material
  fileAssetId?: string;
  url?: string;
  description?: string;
  // article
  body?: string;
  // quiz / listening / reading
  passage?: string;
  audioAssetId?: string;
  questions?: Question[];
  // writing
  prompt?: string;
  minWords?: number;
  rubric?: string;
}

export interface Resource {
  id: string;
  type: ResourceType;
  title: string;
  instructions: string;
  status: ResourceStatus;
  archived: boolean;
  content: ResourceContent;
  createdAt: string;
  updatedAt: string;
}

export interface UploadedFile {
  id: string;
  filename: string;
  contentType: string;
  bytes: number;
  url: string;
}

export const RESOURCE_TYPES: { value: ResourceType; label: string; blurb: string }[] = [
  { value: "material", label: "Material", blurb: "A file or link to share (PDF, slides, doc)." },
  { value: "article", label: "Article", blurb: "Text to read." },
  { value: "quiz", label: "Quiz", blurb: "Auto-graded questions." },
  { value: "listening", label: "Listening task", blurb: "Audio + auto-graded questions." },
  { value: "reading", label: "Reading task", blurb: "A passage + auto-graded questions." },
  { value: "writing", label: "Writing task", blurb: "A prompt the student writes to; you grade it." },
];

export function typeLabel(t: ResourceType): string {
  return RESOURCE_TYPES.find((x) => x.value === t)?.label ?? t;
}

export function isQuizLike(t: ResourceType): boolean {
  return t === "quiz" || t === "listening" || t === "reading";
}

/** Whether a resource type carries a submission flow at all (quiz-like + writing). */
export function hasSubmissionFlow(t: ResourceType): boolean {
  return isQuizLike(t) || t === "writing";
}

/* ------------------- Phase A2/A3 — lesson resources ------------------- */

export type ResourceKind = "material" | "homework";

/** The viewer's own submission status against a homework attachment; "" when
 *  the viewer isn't the student, or hasn't started it yet. */
export type SubmissionStatus = "" | "in_progress" | "submitted" | "graded";

/** The caller's own submission summary for one attached resource. */
export interface SubmissionSummary {
  id: string;
  status: "in_progress" | "submitted" | "graded";
  autoScore: number | null;
  autoMax: number | null;
  teacherScore: number | null;
  submittedAt: string | null;
  gradedAt: string | null;
}

/** One booking attachment with the resource's full display content — the
 *  response shape of POST/GET /v1/bookings/{id}/resources. Student callers
 *  never see `correct` on any question. */
export interface AttachedResource {
  id: string;
  resourceId: string;
  kind: ResourceKind;
  position: number;
  dueAt: string | null;
  type: ResourceType;
  title: string;
  instructions: string;
  resourceStatus: ResourceStatus;
  content: ResourceContent;
  /** The caller's own submission; null for the teacher viewer, or a student who hasn't started. */
  submission: SubmissionSummary | null;
}

export type SubmissionFullStatus = "in_progress" | "submitted" | "graded";

/** A homework submission — the full record, including answers. */
export interface Submission {
  id: string;
  resourceId: string;
  bookingId: string;
  status: SubmissionFullStatus;
  /** Question id -> chosen choice ids (single/multi) or free-text answer
   *  (text, one-element array); writing tasks use the single key "text". */
  answers: Record<string, string[]>;
  autoScore: number | null;
  autoMax: number | null;
  teacherScore: number | null;
  teacherFeedback: string;
  submittedAt: string | null;
  gradedAt: string | null;
  createdAt: string;
  updatedAt: string;
}
