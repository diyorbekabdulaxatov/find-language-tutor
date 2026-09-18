/**
 * Teacher media URLs arrive in one of two shapes: an absolute URL (the seed's
 * placeholder photos, a pasted link) or an API-relative path to the public
 * media route (`/v1/teachers/{slug}/media/avatar`) when the file was
 * uploaded. Only the latter needs the API origin in front of it.
 *
 * `NEXT_PUBLIC_API_URL` is inlined at build time, so this is safe from both
 * server components and the browser.
 */
const apiOrigin = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export function mediaUrl(url: string): string {
  if (url.startsWith("/v1/")) return `${apiOrigin}${url}`;
  return url;
}

/** Owner-side preview of an upload that isn't public yet (`GET /v1/files/{id}`). */
export function isUploadedMedia(url: string): boolean {
  return url.startsWith("/v1/");
}
