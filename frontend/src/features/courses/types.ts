/** View-models for the courses module. camelCase; the wire shapes live in the
 *  generated schema. Phase C1 (authoring) types are above; phase C2 (public
 *  catalog, purchase, enrollment, the student player) types are below. */

import type { Money } from "@/types/teacher";
import type { ResourceContent, ResourceType } from "@/features/resources/types";

export type CourseStatus = "draft" | "published";

export type CourseItemKind = "video" | "resource";

/** A library-list row — course fields only, no curriculum. */
export interface Course {
  id: string;
  title: string;
  subtitle: string;
  description: string;
  coverAssetId: string | null;
  price: Money;
  status: CourseStatus;
  archived: boolean;
  /** Derived aggregate over the course's visible reviews, one decimal.
   *  0 with a reviewCount of 0 means "no ratings yet" — render that, not
   *  zero stars. */
  rating: number;
  reviewCount: number;
  createdAt: string;
  updatedAt: string;
}

/** One curriculum item: an uploaded video, or one of the teacher's own
 *  published resources. `title` is an optional display-title override —
 *  "" means "fall back to the video filename / resource title in the UI". */
export interface CourseItem {
  id: string;
  kind: CourseItemKind;
  title: string;
  videoAssetId: string | null;
  resourceId: string | null;
  position: number;
  /** Free sample lecture, watchable without enrolling. Video items only. */
  isPreview: boolean;
  /** Video length in seconds. 0 = unknown; every surface omits the figure
   *  rather than rendering "0m". */
  durationSeconds: number;
  createdAt: string;
}

export interface CourseSection {
  id: string;
  title: string;
  position: number;
  items: CourseItem[];
  createdAt: string;
  updatedAt: string;
}

/** A course with its full curriculum tree — returned by create/get/update/
 *  publish/archive and every section/item mutation, so the editor can
 *  re-render its whole tree from any response. */
export interface CourseDetail extends Course {
  sections: CourseSection[];
}

/* ------------------- Phase C2 — catalog, purchase, player ------------------- */

export type CourseSort = "newest" | "price_asc" | "price_desc" | "rating";

/** Light teacher summary embedded in catalog rows. */
export interface CourseTeacherSummary {
  id: string;
  displayName: string;
  slug: string;
}

/** One public catalog row — course + teacher summary + curriculum size. */
export interface CourseCatalogEntry {
  id: string;
  title: string;
  subtitle: string;
  coverAssetId: string | null;
  price: Money;
  teacher: CourseTeacherSummary;
  sectionCount: number;
  itemCount: number;
  /** Summed length of every video item. 0 = unknown. */
  totalDurationSeconds: number;
  /** At least one lecture is free to watch. */
  hasPreview: boolean;
  rating: number;
  reviewCount: number;
}

/** A curriculum item's pre-purchase outline — title only, no content. */
export interface CourseCatalogItemOutline {
  id: string;
  kind: CourseItemKind;
  title: string;
  position: number;
  isPreview: boolean;
  durationSeconds: number;
}

export interface CourseCatalogSectionOutline {
  id: string;
  title: string;
  position: number;
  items: CourseCatalogItemOutline[];
}

/** A course's public landing page — course + teacher summary, curriculum
 *  outline only, and the viewer's relationship to it (false/false when the
 *  viewer is anonymous). */
export interface CourseCatalogDetail {
  id: string;
  title: string;
  subtitle: string;
  description: string;
  coverAssetId: string | null;
  price: Money;
  teacher: CourseTeacherSummary;
  sections: CourseCatalogSectionOutline[];
  isEnrolled: boolean;
  isOwner: boolean;
  /** Headline "N lectures · H hours". */
  itemCount: number;
  totalDurationSeconds: number;
  rating: number;
  reviewCount: number;
  /** The viewer's own review, so the page can offer "edit" rather than a
   *  "write one" button that would 409. Null for an anonymous viewer, the
   *  owner, a non-buyer, or a buyer who hasn't reviewed yet. */
  myReview: CourseReview | null;
}

/** One buyer's standing opinion of a course. */
export interface CourseReview {
  id: string;
  rating: number;
  comment: string;
  studentDisplayName: string;
  createdAt: string;
  /** Differs from createdAt once the author has revised it. */
  updatedAt: string;
}

/** A page of a course's public reviews, plus the star histogram over ALL
 *  visible reviews (index 0 = 1★ … index 4 = 5★) so the bars don't move as
 *  the reader pages through. */
export interface CourseReviewPage {
  reviews: CourseReview[];
  total: number;
  breakdown: number[];
}

export type EnrollmentSource = "purchase" | "free";

/** One row of the caller's "my learning" list. */
export interface CourseEnrollment {
  id: string;
  course: Course;
  source: EnrollmentSource;
  amountPaid: Money;
  /** completed_items / total_items * 100, 0 for an empty course. */
  progressPercent: number;
  createdAt: string;
}

export type CourseItemProgressStatus = "in_progress" | "completed";

export interface CourseItemProgress {
  status: CourseItemProgressStatus;
  videoPositionSeconds: number;
  completedAt: string | null;
}

/** A published resource's student-safe view (answers already stripped),
 *  embedded in a resource-kind curriculum item. */
export interface CourseResourceView {
  id: string;
  type: ResourceType;
  title: string;
  instructions: string;
  content: ResourceContent;
}

/** One curriculum item in the enrolled-student player: a video (fetch bytes
 *  via GET /v1/files/{id}) or an embedded resource (student-safe content). */
export interface CourseLearnItem {
  id: string;
  kind: CourseItemKind;
  title: string;
  position: number;
  videoAssetId: string | null;
  resource: CourseResourceView | null;
  progress: CourseItemProgress;
}

export interface CourseLearnSection {
  id: string;
  title: string;
  items: CourseLearnItem[];
}

/** The full curriculum tree for an enrolled student (or the owner previewing
 *  their own course). `enrollmentId` is null only in the owner-preview case. */
export interface CourseLearnDetail {
  id: string;
  title: string;
  enrollmentId: string | null;
  sections: CourseLearnSection[];
}
