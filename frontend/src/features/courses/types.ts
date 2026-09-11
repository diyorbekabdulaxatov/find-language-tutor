/** View-models for the course-authoring library. camelCase; the wire shapes
 *  live in the generated schema. Authoring only — no catalog/purchase shapes
 *  here (phase C1). */

import type { Money } from "@/types/teacher";

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
