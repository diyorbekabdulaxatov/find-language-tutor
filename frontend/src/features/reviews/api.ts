/**
 * Reviews data-access (browser).
 *
 * NOTE: `GET /v1/teachers/{slug}/reviews` and `POST /v1/bookings/{id}/review`
 * are being added to the backend (Phase 6). Until they land in openapi.yaml +
 * `npm run gen:api`, this uses `authedFetch` with hand-written wire types; swap
 * to the typed `browserApi` once the schema regenerates.
 */

import { authedFetch } from "@/features/auth/browser-client";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface Review {
  id: string;
  rating: number;
  comment: string;
  createdAt: string;
  studentDisplayName: string;
}

export interface ReviewPage {
  reviews: Review[];
  total: number;
}

export class ReviewError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ReviewError";
    this.code = code;
    this.status = status;
  }
}

type ErrorBody = { error?: { code?: string; message?: string } };

async function call<T>(
  path: string,
  init: RequestInit,
  fallback: string,
): Promise<T> {
  const res = await authedFetch(`${baseUrl}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
  const body =
    res.status === 204 ? undefined : await res.json().catch(() => undefined);
  if (!res.ok) {
    const e = body as ErrorBody | undefined;
    throw new ReviewError(
      e?.error?.message ?? fallback,
      e?.error?.code ?? "unknown",
      res.status,
    );
  }
  return body as T;
}

interface WireReview {
  id: string;
  rating: number;
  comment: string;
  created_at: string;
  student_display_name: string;
}

const toReview = (r: WireReview): Review => ({
  id: r.id,
  rating: r.rating,
  comment: r.comment,
  createdAt: r.created_at,
  studentDisplayName: r.student_display_name,
});

export async function getTeacherReviews(
  slug: string,
  page = 1,
  pageSize = 10,
): Promise<ReviewPage> {
  const q = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  });
  const w = await call<{ reviews: WireReview[]; total: number }>(
    `/v1/teachers/${encodeURIComponent(slug)}/reviews?${q}`,
    { method: "GET" },
    "Could not load reviews.",
  );
  return { reviews: w.reviews.map(toReview), total: w.total };
}

export async function submitReview(
  bookingId: string,
  input: { rating: number; comment: string },
): Promise<void> {
  await call<unknown>(
    `/v1/bookings/${encodeURIComponent(bookingId)}/review`,
    { method: "POST", body: JSON.stringify(input) },
    "Could not submit your review.",
  );
}
