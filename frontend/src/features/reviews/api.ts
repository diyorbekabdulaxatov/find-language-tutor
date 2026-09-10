/**
 * Reviews data-access (browser). `GET /v1/teachers/{slug}/reviews` is public;
 * `POST /v1/bookings/{id}/review` needs the student's token. Both go through
 * the generated `browserApi`.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";

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

function toError(error: unknown, status: number, fallback: string): ReviewError {
  const b = error as ErrorBody | undefined;
  return new ReviewError(
    b?.error?.message ?? fallback,
    b?.error?.code ?? "unknown",
    status,
  );
}

const toReview = (r: components["schemas"]["ReviewListItem"]): Review => ({
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
  const { data, error, response } = await browserApi.GET(
    "/v1/teachers/{slug}/reviews",
    { params: { path: { slug }, query: { page, page_size: pageSize } } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not load reviews.");
  }
  return { reviews: data.reviews.map(toReview), total: data.total };
}

export async function submitReview(
  bookingId: string,
  input: { rating: number; comment: string },
): Promise<void> {
  const { error, response } = await browserApi.POST(
    "/v1/bookings/{id}/review",
    {
      params: { path: { id: bookingId } },
      body: { rating: input.rating, comment: input.comment },
    },
  );
  if (error) {
    throw toError(error, response.status, "Could not submit your review.");
  }
}
