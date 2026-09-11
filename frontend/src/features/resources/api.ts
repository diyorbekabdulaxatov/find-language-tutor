/**
 * Resource-library data access (browser). Wraps the generated client, maps
 * wire ↔ view-model, and throws a typed ResourceError on failure.
 */

import { authedFetch, browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type {
  AttachedResource,
  Resource,
  ResourceContent,
  ResourceKind,
  ResourceStatus,
  ResourceType,
  UploadedFile,
} from "./types";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ResourceError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ResourceError";
    this.code = code;
    this.status = status;
  }
}

type ErrBody = { error?: { code?: string; message?: string } };
function toErr(e: unknown, status: number, fallback: string): ResourceError {
  const b = e as ErrBody | undefined;
  return new ResourceError(b?.error?.message ?? fallback, b?.error?.code ?? "unknown", status);
}

/* -------------------------- wire ↔ view-model -------------------------- */

type WireResource = components["schemas"]["Resource"];
type WireContent = components["schemas"]["ResourceContent"];

function toContent(c: WireContent): ResourceContent {
  return {
    fileAssetId: c.file_asset_id,
    url: c.url,
    description: c.description,
    body: c.body,
    passage: c.passage,
    audioAssetId: c.audio_asset_id,
    questions: c.questions?.map((q) => ({
      id: q.id,
      prompt: q.prompt,
      kind: q.kind,
      choices: q.choices ?? [],
      // Stripped from the wire for a student viewer (never leak the answer key).
      correct: q.correct ?? [],
      points: q.points,
    })),
    prompt: c.prompt,
    minWords: c.min_words,
    rubric: c.rubric,
  };
}

function fromContent(c: ResourceContent): WireContent {
  return {
    file_asset_id: c.fileAssetId || undefined,
    url: c.url || undefined,
    description: c.description || undefined,
    body: c.body || undefined,
    passage: c.passage || undefined,
    audio_asset_id: c.audioAssetId || undefined,
    questions: c.questions?.map((q) => ({
      id: q.id,
      prompt: q.prompt,
      kind: q.kind,
      choices: q.kind === "text" ? undefined : q.choices,
      correct: q.correct,
      points: q.points,
    })),
    prompt: c.prompt || undefined,
    min_words: c.minWords || undefined,
    rubric: c.rubric || undefined,
  };
}

function toResource(r: WireResource): Resource {
  return {
    id: r.id,
    type: r.type,
    title: r.title,
    instructions: r.instructions,
    status: r.status,
    archived: r.archived,
    content: toContent(r.content),
    createdAt: r.created_at,
    updatedAt: r.updated_at,
  };
}

/* ------------------------------- calls -------------------------------- */

export async function listResources(opts: {
  type?: ResourceType;
  status?: ResourceStatus;
  archived?: boolean;
  page?: number;
}): Promise<{ resources: Resource[]; total: number }> {
  const { data, error, response } = await browserApi.GET("/v1/resources", {
    params: {
      query: {
        type: opts.type,
        status: opts.status,
        archived: opts.archived ? "true" : undefined,
        page: opts.page,
      },
    },
  });
  if (error || !data) {
    throw toErr(error, response.status, "Could not load your resources.");
  }
  return { resources: data.resources.map(toResource), total: data.total };
}

export async function getResource(id: string): Promise<Resource> {
  const { data, error, response } = await browserApi.GET("/v1/resources/{id}", {
    params: { path: { id } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load that resource.");
  return toResource(data);
}

export async function createResource(input: {
  type: ResourceType;
  title: string;
  instructions: string;
  publish: boolean;
  content: ResourceContent;
}): Promise<Resource> {
  const { data, error, response } = await browserApi.POST("/v1/resources", {
    body: {
      type: input.type,
      title: input.title,
      instructions: input.instructions,
      publish: input.publish,
      content: fromContent(input.content),
    },
  });
  if (error || !data) throw toErr(error, response.status, "Could not create the resource.");
  return toResource(data);
}

export async function updateResource(
  id: string,
  input: { title: string; instructions: string; content: ResourceContent },
): Promise<Resource> {
  const { data, error, response } = await browserApi.PATCH("/v1/resources/{id}", {
    params: { path: { id } },
    body: {
      title: input.title,
      instructions: input.instructions,
      content: fromContent(input.content),
    },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save your changes.");
  return toResource(data);
}

async function action(id: string, verb: "publish" | "unpublish" | "archive" | "unarchive"): Promise<Resource> {
  const { data, error, response } = await browserApi.POST(
    `/v1/resources/{id}/${verb}` as "/v1/resources/{id}/publish",
    { params: { path: { id } } },
  );
  if (error || !data) throw toErr(error, response.status, `Could not ${verb} the resource.`);
  return toResource(data);
}

export const publishResource = (id: string) => action(id, "publish");
export const unpublishResource = (id: string) => action(id, "unpublish");
export const archiveResource = (id: string) => action(id, "archive");
export const unarchiveResource = (id: string) => action(id, "unarchive");

export async function deleteResource(id: string): Promise<void> {
  const { error, response } = await browserApi.DELETE("/v1/resources/{id}", {
    params: { path: { id } },
  });
  if (error) throw toErr(error, response.status, "Could not delete the resource.");
}

/** Upload a file. Uses authedFetch directly — openapi-fetch doesn't do multipart. */
export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file);
  const res = await authedFetch(`${baseUrl}/v1/uploads`, { method: "POST", body: form });
  const body = await res.json().catch(() => undefined);
  if (!res.ok) {
    throw toErr(body, res.status, "Could not upload the file.");
  }
  return {
    id: body.id,
    filename: body.filename,
    contentType: body.content_type,
    bytes: body.bytes,
    url: `${baseUrl}${body.url}`,
  };
}

/**
 * The download URL for a stored file id — NOT directly usable as an `<a href>`
 * or `<audio src>`: GET /v1/files/{id} requires a bearer token via
 * `requireAuth`, which only `authedFetch` attaches. A plain browser
 * navigation or media-element fetch sends no Authorization header and gets a
 * 401. Kept for callers that only need the path (e.g. logging); for actually
 * displaying/downloading a file, use `fetchFileObjectUrl` instead.
 */
export function fileUrl(id: string): string {
  return `${baseUrl}/v1/files/${id}`;
}

/**
 * Fetches a stored file's bytes through the authenticated endpoint and
 * returns a browser object URL usable as an `<a href>` / `<audio src>`. The
 * caller owns the URL's lifecycle — `URL.revokeObjectURL` it when done.
 */
export async function fetchFileObjectUrl(id: string): Promise<string> {
  const res = await authedFetch(fileUrl(id));
  if (!res.ok) {
    const body = await res.json().catch(() => undefined);
    throw toErr(body, res.status, "Could not load that file.");
  }
  const blob = await res.blob();
  return URL.createObjectURL(blob);
}

/* ------------------- Phase A2/A3 — lesson resources -------------------- */

type WireAttachedResource = components["schemas"]["AttachedResource"];

function toAttachedResource(a: WireAttachedResource): AttachedResource {
  return {
    id: a.id,
    resourceId: a.resource_id,
    kind: a.kind,
    position: a.position,
    dueAt: a.due_at,
    type: a.type,
    title: a.title,
    instructions: a.instructions,
    resourceStatus: a.resource_status,
    content: toContent(a.content),
    submission: a.submission
      ? {
          id: a.submission.id,
          status: a.submission.status,
          autoScore: a.submission.auto_score,
          autoMax: a.submission.auto_max,
          teacherScore: a.submission.teacher_score,
          submittedAt: a.submission.submitted_at,
          gradedAt: a.submission.graded_at,
        }
      : null,
  };
}

/** Teacher-owner only. Attaches one of the teacher's own published resources
 *  to a lesson, as material or homework (homework is rejected server-side
 *  for `material` / `article` types). */
export async function attachResourceToBooking(
  bookingId: string,
  input: { resourceId: string; kind: ResourceKind; dueAt?: string },
): Promise<AttachedResource> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/resources",
    {
      params: { path: { id: bookingId } },
      body: {
        resource_id: input.resourceId,
        kind: input.kind,
        due_at: input.dueAt,
      },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not attach that resource.");
  return toAttachedResource(data);
}

/** Participant-only. The teacher sees full content (including answers); the
 *  student sees `correct` stripped from every question plus their own
 *  submission summary. */
export async function listBookingAttachments(bookingId: string): Promise<AttachedResource[]> {
  const { data, error, response } = await browserApi.GET("/v1/bookings/{id}/resources", {
    params: { path: { id: bookingId } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load the lesson's resources.");
  return data.attachments.map(toAttachedResource);
}

/** Teacher-owner only. Submissions already filed against the resource are kept. */
export async function detachBookingResource(bookingId: string, attachmentId: string): Promise<void> {
  const { error, response } = await browserApi.DELETE("/v1/bookings/{id}/resources/{attachmentId}", {
    params: { path: { id: bookingId, attachmentId } },
  });
  if (error) throw toErr(error, response.status, "Could not remove that resource.");
}
