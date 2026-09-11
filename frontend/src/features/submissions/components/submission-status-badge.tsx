import { Badge } from "@/components/ui/badge";
import type { SubmissionStatus } from "@/features/resources/types";

/**
 * A small status pill for one student's homework attempt. Shows a score when
 * one exists — auto-graded first (quiz-like), else a teacher's score once
 * graded (writing).
 */
export function SubmissionStatusBadge({
  status,
  autoScore,
  autoMax,
  teacherScore,
}: {
  status: SubmissionStatus;
  autoScore?: number | null;
  autoMax?: number | null;
  teacherScore?: number | null;
}) {
  if (status === "" ) {
    return (
      <Badge variant="outline" className="text-muted-foreground">
        Not started
      </Badge>
    );
  }
  if (status === "in_progress") {
    return (
      <Badge variant="secondary" className="text-star">
        In progress
      </Badge>
    );
  }
  if (status === "submitted") {
    return (
      <Badge variant="secondary" className="text-star">
        Submitted, awaiting feedback
      </Badge>
    );
  }
  // graded
  const hasAuto = autoScore != null && autoMax != null;
  return (
    <Badge className="bg-primary/15 text-primary">
      Graded{hasAuto ? ` · ${autoScore}/${autoMax}` : teacherScore != null ? ` · ${teacherScore}` : ""}
    </Badge>
  );
}
