/**
 * `POST /v1/uploads` with a progress callback. `fetch` can't report upload
 * progress, and an intro video can be hundreds of megabytes, so this one
 * call goes over XMLHttpRequest — with the same auth contract as
 * `authedFetch`: Bearer from the in-memory store, Accept-Language from
 * <html lang>, and exactly one refresh-and-retry on a 401.
 */

import { getAccessToken } from "@/features/auth/auth-store";
import { refreshSession } from "@/features/auth/browser-client";
import { ProfileError } from "@/features/dashboard/api";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface UploadedAsset {
  id: string;
  filename: string;
  contentType: string;
  bytes: number;
}

export function uploadWithProgress(
  file: File,
  onProgress: (fraction: number) => void,
  signal?: AbortSignal,
): Promise<UploadedAsset> {
  return attempt(file, onProgress, signal, true);
}

async function attempt(
  file: File,
  onProgress: (fraction: number) => void,
  signal: AbortSignal | undefined,
  mayRefresh: boolean,
): Promise<UploadedAsset> {
  const { status, body } = await send(file, onProgress, signal);
  if (status === 401 && mayRefresh && (await refreshSession())) {
    return attempt(file, onProgress, signal, false);
  }
  const parsed = parse(body);
  if (status !== 201) {
    throw new ProfileError(
      parsed?.error?.message ?? "Could not upload the file.",
      parsed?.error?.code ?? "unknown",
      status,
    );
  }
  return {
    id: parsed.id,
    filename: parsed.filename,
    contentType: parsed.content_type,
    bytes: parsed.bytes,
  };
}

type Wire = {
  id: string;
  filename: string;
  content_type: string;
  bytes: number;
  error?: { code?: string; message?: string };
};

function parse(body: string): Wire {
  try {
    return JSON.parse(body) as Wire;
  } catch {
    return {} as Wire;
  }
}

function send(
  file: File,
  onProgress: (fraction: number) => void,
  signal?: AbortSignal,
): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${baseUrl}/v1/uploads`);
    xhr.withCredentials = true;
    const token = getAccessToken();
    if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    const lang = typeof document !== "undefined" ? document.documentElement.lang : "";
    if (lang) xhr.setRequestHeader("Accept-Language", lang);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total);
    };
    xhr.onload = () => resolve({ status: xhr.status, body: xhr.responseText });
    xhr.onerror = () => reject(new ProfileError("Could not upload the file.", "network", 0));
    xhr.onabort = () => reject(new ProfileError("Upload cancelled.", "aborted", 0));
    signal?.addEventListener("abort", () => xhr.abort(), { once: true });

    const form = new FormData();
    form.append("file", file);
    xhr.send(form);
  });
}
