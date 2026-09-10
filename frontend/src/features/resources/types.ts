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
