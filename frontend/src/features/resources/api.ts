/**
 * Resource-library data access (browser). Wraps the generated client, maps
 * wire ↔ view-model, and throws a typed ResourceError on failure.
 */

import { authedFetch, browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type {
  Resource,
  ResourceContent,
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
      correct: q.correct,
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

/** The download URL for a stored file id (goes through the auth'd endpoint). */
export function fileUrl(id: string): string {
  return `${baseUrl}/v1/files/${id}`;
}
