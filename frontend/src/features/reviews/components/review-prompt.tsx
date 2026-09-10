"use client";

import { useState } from "react";
import { ReviewError, submitReview } from "@/features/reviews/api";
import { StarInput, Stars } from "./star-rating";
import { Button } from "@/components/ui/button";

/** Post-lesson review form, shown on a completed booking the student hasn't reviewed. */
export function ReviewPrompt({
  bookingId,
  teacherName,
  onSubmitted,
}: {
  bookingId: string;
  teacherName: string;
  onSubmitted: () => void;
}) {
  const [rating, setRating] = useState(0);
  const [comment, setComment] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (rating < 1) {
      setError("Pick a star rating.");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await submitReview(bookingId, { rating, comment: comment.trim() });
      onSubmitted();
    } catch (err) {
      setSubmitting(false);
      setError(
        err instanceof ReviewError
          ? err.message
          : "Could not submit your review. Please try again.",
      );
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className="mt-4 flex flex-col gap-3 rounded-2xl border border-border bg-card p-6"
    >
      <h2 className="font-display text-lg">How was your lesson with {teacherName}?</h2>
      <StarInput value={rating} onChange={setRating} />
      <textarea
        rows={3}
        placeholder="Share what went well (optional)"
        className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        value={comment}
        maxLength={2000}
        onChange={(e) => setComment(e.target.value)}
      />
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <Button type="submit" disabled={submitting} className="self-start">
        {submitting ? "Submitting…" : "Post review"}
      </Button>
    </form>
  );
}

/** Read-only display of a review already left on a booking. */
export function BookingReview({
  rating,
  comment,
}: {
  rating: number;
  comment: string;
}) {
  return (
    <div className="mt-4 rounded-2xl border border-border bg-card p-6">
      <h2 className="font-display text-lg">Your review</h2>
      <Stars value={rating} className="mt-2" />
      {comment && (
        <p className="mt-2 text-sm leading-relaxed text-foreground/90">{comment}</p>
      )}
    </div>
  );
}
