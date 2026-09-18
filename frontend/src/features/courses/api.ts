/**
 * Course-authoring data access (browser). Wraps the generated client, maps
 * wire ↔ view-model, and throws a typed CourseError on failure. Mirrors
 * `features/resources/api.ts`'s conventions.
 */

import { api } from "@/lib/api/client";
import { baseUrl, browserApi } from "@/features/auth/browser-client";
import { toContent } from "@/features/resources/api";
import type { components } from "@/lib/api/schema";
import type { Money } from "@/types/teacher";
import type {
  Course,
  CourseCatalogDetail,
  CourseCatalogEntry,
  CourseReview,
  CourseReviewPage,
  CourseCatalogItemOutline,
  CourseCatalogSectionOutline,
  CourseDetail,
  CourseEnrollment,
  CourseItem,
  CourseItemKind,
  CourseItemProgress,
  CourseLearnDetail,
  CourseLearnItem,
  CourseLearnSection,
  CourseResourceView,
  CourseSection,
  CourseSort,
  CourseStatus,
  CourseTeacherSummary,
} from "./types";

export class CourseError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "CourseError";
    this.code = code;
    this.status = status;
  }
}

type ErrBody = { error?: { code?: string; message?: string } };
function toErr(e: unknown, status: number, fallback: string): CourseError {
  const b = e as ErrBody | undefined;
  return new CourseError(b?.error?.message ?? fallback, b?.error?.code ?? "unknown", status);
}

/* -------------------------- wire ↔ view-model -------------------------- */

type WireMoney = components["schemas"]["Money"];
type WireCourse = components["schemas"]["Course"];
type WireCourseDetail = components["schemas"]["CourseDetail"];
type WireSection = components["schemas"]["CourseSection"];
type WireItem = components["schemas"]["CourseItem"];

function toMoney(m: WireMoney): Money {
  return { amountMinor: m.amount_minor, currency: m.currency };
}

function toCourse(c: WireCourse): Course {
  return {
    id: c.id,
    title: c.title,
    subtitle: c.subtitle,
    description: c.description,
    coverAssetId: c.cover_asset_id,
    price: toMoney(c.price),
    status: c.status,
    archived: c.archived,
    rating: c.rating,
    reviewCount: c.review_count,
    createdAt: c.created_at,
    updatedAt: c.updated_at,
  };
}

function toItem(i: WireItem): CourseItem {
  return {
    id: i.id,
    kind: i.kind,
    title: i.title,
    videoAssetId: i.video_asset_id,
    resourceId: i.resource_id,
    position: i.position,
    isPreview: i.is_preview,
    durationSeconds: i.duration_seconds,
    createdAt: i.created_at,
  };
}

function toSection(s: WireSection): CourseSection {
  return {
    id: s.id,
    title: s.title,
    position: s.position,
    items: s.items.map(toItem),
    createdAt: s.created_at,
    updatedAt: s.updated_at,
  };
}

function toCourseDetail(c: WireCourseDetail): CourseDetail {
  return { ...toCourse(c), sections: c.sections.map(toSection) };
}

/* ------------------------------- calls -------------------------------- */

export async function listCourses(opts: {
  status?: CourseStatus;
  archived?: boolean;
  page?: number;
}): Promise<{ courses: Course[]; total: number }> {
  const { data, error, response } = await browserApi.GET("/v1/courses", {
    params: {
      query: {
        status: opts.status,
        archived: opts.archived ? "true" : undefined,
        page: opts.page,
      },
    },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load your courses.");
  return { courses: data.courses.map(toCourse), total: data.total };
}

export async function createCourse(input: {
  title: string;
  subtitle?: string;
  description?: string;
  priceAmountMinor?: number;
  priceCurrency?: "UZS";
}): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.POST("/v1/courses", {
    body: {
      title: input.title,
      subtitle: input.subtitle || undefined,
      description: input.description || undefined,
      price_amount_minor: input.priceAmountMinor ?? 0,
      price_currency: input.priceCurrency ?? "UZS",
    },
  });
  if (error || !data) throw toErr(error, response.status, "Could not create the course.");
  return toCourseDetail(data);
}

export async function getCourse(id: string): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.GET("/v1/courses/{id}", {
    params: { path: { id } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load that course.");
  return toCourseDetail(data);
}

export async function updateCourse(
  id: string,
  input: {
    title: string;
    subtitle: string;
    description: string;
    coverAssetId: string | null;
    priceAmountMinor: number;
    priceCurrency: "UZS";
  },
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PATCH("/v1/courses/{id}", {
    params: { path: { id } },
    body: {
      title: input.title,
      subtitle: input.subtitle,
      description: input.description,
      cover_asset_id: input.coverAssetId,
      price_amount_minor: input.priceAmountMinor,
      price_currency: input.priceCurrency,
    },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save your changes.");
  return toCourseDetail(data);
}

async function courseAction(
  id: string,
  verb: "publish" | "unpublish" | "archive" | "unarchive",
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.POST(
    `/v1/courses/{id}/${verb}` as "/v1/courses/{id}/publish",
    { params: { path: { id } } },
  );
  if (error || !data) throw toErr(error, response.status, `Could not ${verb} the course.`);
  return toCourseDetail(data);
}

export const setCoursePublished = (id: string, publish: boolean) =>
  courseAction(id, publish ? "publish" : "unpublish");
export const setCourseArchived = (id: string, archived: boolean) =>
  courseAction(id, archived ? "archive" : "unarchive");

export async function deleteCourse(id: string): Promise<void> {
  const { error, response } = await browserApi.DELETE("/v1/courses/{id}", {
    params: { path: { id } },
  });
  if (error) throw toErr(error, response.status, "Could not delete the course.");
}

export async function addSection(courseId: string, title: string): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.POST("/v1/courses/{id}/sections", {
    params: { path: { id: courseId } },
    body: { title },
  });
  if (error || !data) throw toErr(error, response.status, "Could not add that section.");
  return toCourseDetail(data);
}

export async function renameSection(
  courseId: string,
  sectionId: string,
  title: string,
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PATCH("/v1/courses/{id}/sections/{sectionId}", {
    params: { path: { id: courseId, sectionId } },
    body: { title },
  });
  if (error || !data) throw toErr(error, response.status, "Could not rename that section.");
  return toCourseDetail(data);
}

export async function deleteSection(courseId: string, sectionId: string): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.DELETE("/v1/courses/{id}/sections/{sectionId}", {
    params: { path: { id: courseId, sectionId } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not delete that section.");
  return toCourseDetail(data);
}

export async function reorderSections(
  courseId: string,
  sectionIds: string[],
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PUT("/v1/courses/{id}/sections/reorder", {
    params: { path: { id: courseId } },
    body: { section_ids: sectionIds },
  });
  if (error || !data) throw toErr(error, response.status, "Could not reorder the sections.");
  return toCourseDetail(data);
}

export async function addItem(
  courseId: string,
  sectionId: string,
  input: {
    kind: CourseItemKind;
    title?: string;
    videoAssetId?: string;
    resourceId?: string;
    isPreview?: boolean;
    durationSeconds?: number;
  },
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.POST(
    "/v1/courses/{id}/sections/{sectionId}/items",
    {
      params: { path: { id: courseId, sectionId } },
      body: {
        kind: input.kind,
        title: input.title || undefined,
        video_asset_id: input.videoAssetId,
        resource_id: input.resourceId,
        is_preview: input.isPreview,
        duration_seconds: input.durationSeconds,
      },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not add that item.");
  return toCourseDetail(data);
}

/** Edits an item's title and, for a video item, its preview flag / duration.
 *  `isPreview` and `durationSeconds` are merge-on-write on the backend: leave
 *  one undefined to keep the stored value. */
export async function updateItem(
  courseId: string,
  sectionId: string,
  itemId: string,
  input: { title: string; isPreview?: boolean; durationSeconds?: number },
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PATCH(
    "/v1/courses/{id}/sections/{sectionId}/items/{itemId}",
    {
      params: { path: { id: courseId, sectionId, itemId } },
      body: {
        title: input.title,
        is_preview: input.isPreview,
        duration_seconds: input.durationSeconds,
      },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not update that item.");
  return toCourseDetail(data);
}

export async function deleteItem(
  courseId: string,
  sectionId: string,
  itemId: string,
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.DELETE(
    "/v1/courses/{id}/sections/{sectionId}/items/{itemId}",
    { params: { path: { id: courseId, sectionId, itemId } } },
  );
  if (error || !data) throw toErr(error, response.status, "Could not delete that item.");
  return toCourseDetail(data);
}

export async function reorderItems(
  courseId: string,
  sectionId: string,
  itemIds: string[],
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PUT(
    "/v1/courses/{id}/sections/{sectionId}/items/reorder",
    {
      params: { path: { id: courseId, sectionId } },
      body: { item_ids: itemIds },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not reorder the items.");
  return toCourseDetail(data);
}

/* ===================== Phase C2 — catalog, purchase, player ===================== */

/** Direct, unauthenticated URL for a course's cover image — usable straight
 *  in an `<img src>` from either a server- or client-rendered page. */
export function courseCoverUrl(courseId: string): string {
  return `${baseUrl}/v1/courses/${courseId}/cover`;
}

/** Public, unauthenticated stream of a free preview lecture. 404s unless the
 *  item is flagged `is_preview` and its course is on the storefront, so it is
 *  safe to build the URL for any outline item and let the server decide. */
export function coursePreviewUrl(courseId: string, itemId: string): string {
  return `${baseUrl}/v1/courses/${courseId}/items/${itemId}/preview`;
}

type WireTeacherSummary = components["schemas"]["CourseTeacherSummary"];
type WireCatalogEntry = components["schemas"]["CourseCatalogEntry"];
type WireCatalogItemOutline = components["schemas"]["CourseCatalogItemOutline"];
type WireCatalogSectionOutline = components["schemas"]["CourseCatalogSectionOutline"];
type WireCatalogDetail = components["schemas"]["CourseCatalogDetail"];
type WireEnrollment = components["schemas"]["CourseEnrollment"];
type WireItemProgress = components["schemas"]["CourseItemProgress"];
type WireResourceView = components["schemas"]["CourseResourceView"];
type WireLearnItem = components["schemas"]["CourseLearnItem"];
type WireLearnSection = components["schemas"]["CourseLearnSection"];
type WireLearnDetail = components["schemas"]["CourseLearnDetail"];

function toTeacherSummary(t: WireTeacherSummary): CourseTeacherSummary {
  return { id: t.id, displayName: t.display_name, slug: t.slug };
}

function toCatalogEntry(c: WireCatalogEntry): CourseCatalogEntry {
  return {
    id: c.id,
    title: c.title,
    subtitle: c.subtitle,
    coverAssetId: c.cover_asset_id,
    price: toMoney(c.price),
    teacher: toTeacherSummary(c.teacher),
    sectionCount: c.section_count,
    itemCount: c.item_count,
    totalDurationSeconds: c.total_duration_seconds,
    hasPreview: c.has_preview,
    rating: c.rating,
    reviewCount: c.review_count,
  };
}

function toCatalogItemOutline(i: WireCatalogItemOutline): CourseCatalogItemOutline {
  return {
    id: i.id,
    kind: i.kind,
    title: i.title,
    position: i.position,
    isPreview: i.is_preview,
    durationSeconds: i.duration_seconds,
  };
}

function toCatalogSectionOutline(s: WireCatalogSectionOutline): CourseCatalogSectionOutline {
  return {
    id: s.id,
    title: s.title,
    position: s.position,
    items: s.items.map(toCatalogItemOutline),
  };
}

function toCatalogDetail(c: WireCatalogDetail): CourseCatalogDetail {
  return {
    id: c.id,
    title: c.title,
    subtitle: c.subtitle,
    description: c.description,
    coverAssetId: c.cover_asset_id,
    price: toMoney(c.price),
    teacher: toTeacherSummary(c.teacher),
    sections: c.sections.map(toCatalogSectionOutline),
    isEnrolled: c.is_enrolled,
    isOwner: c.is_owner,
    itemCount: c.item_count,
    totalDurationSeconds: c.total_duration_seconds,
    rating: c.rating,
    reviewCount: c.review_count,
    myReview: c.my_review ? toCourseReview(c.my_review) : null,
  };
}

/* --------------------------- reviews (phase D2) --------------------------- */

type WireCourseReview = components["schemas"]["CourseReview"];

function toCourseReview(r: WireCourseReview): CourseReview {
  return {
    id: r.id,
    rating: r.rating,
    comment: r.comment,
    studentDisplayName: r.student_display_name,
    createdAt: r.created_at,
    updatedAt: r.updated_at,
  };
}

/** A course's public review list. Unauthenticated — uses `browserApi` only
 *  because the landing page that renders it is a client island. */
export async function listCourseReviews(
  courseId: string,
  page = 1,
): Promise<CourseReviewPage> {
  const { data, error, response } = await browserApi.GET("/v1/courses/{id}/reviews", {
    params: { path: { id: courseId }, query: { page } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load the reviews.");
  return {
    reviews: data.reviews.map(toCourseReview),
    total: data.total,
    breakdown: data.breakdown,
  };
}

/** Write the caller's review. 409 when they already have one — the caller
 *  should have used `updateCourseReview`, which the landing page decides
 *  between using `myReview`. */
export async function createCourseReview(
  courseId: string,
  input: { rating: number; comment: string },
): Promise<CourseReview> {
  const { data, error, response } = await browserApi.POST("/v1/courses/{id}/review", {
    params: { path: { id: courseId } },
    body: { rating: input.rating, comment: input.comment },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save your review.");
  return toCourseReview(data);
}

/** Revise the caller's own review. */
export async function updateCourseReview(
  courseId: string,
  input: { rating: number; comment: string },
): Promise<CourseReview> {
  const { data, error, response } = await browserApi.PATCH("/v1/courses/{id}/review", {
    params: { path: { id: courseId } },
    body: { rating: input.rating, comment: input.comment },
  });
  if (error || !data) throw toErr(error, response.status, "Could not save your review.");
  return toCourseReview(data);
}

function toEnrollment(e: WireEnrollment): CourseEnrollment {
  return {
    id: e.id,
    course: toCourse(e.course),
    source: e.source,
    amountPaid: toMoney(e.amount_paid),
    progressPercent: e.progress_percent,
    createdAt: e.created_at,
  };
}

function toItemProgress(p: WireItemProgress): CourseItemProgress {
  return {
    status: p.status,
    videoPositionSeconds: p.video_position_seconds,
    completedAt: p.completed_at,
  };
}

function toResourceView(r: WireResourceView): CourseResourceView {
  return {
    id: r.id,
    type: r.type,
    title: r.title,
    instructions: r.instructions,
    content: toContent(r.content),
  };
}

function toLearnItem(i: WireLearnItem): CourseLearnItem {
  return {
    id: i.id,
    kind: i.kind,
    title: i.title,
    position: i.position,
    videoAssetId: i.video_asset_id,
    resource: i.resource ? toResourceView(i.resource) : null,
    progress: toItemProgress(i.progress),
  };
}

function toLearnSection(s: WireLearnSection): CourseLearnSection {
  return { id: s.id, title: s.title, items: s.items.map(toLearnItem) };
}

function toLearnDetail(c: WireLearnDetail): CourseLearnDetail {
  return {
    id: c.id,
    title: c.title,
    enrollmentId: c.enrollment_id,
    sections: c.sections.map(toLearnSection),
  };
}

export interface CourseCatalogParams {
  q?: string;
  maxPriceMinor?: number;
  page?: number;
  pageSize?: number;
  sort?: CourseSort;
}

/** Public catalog list. Unauthenticated — for Server Component reads. */
export async function listCourseCatalog(
  params: CourseCatalogParams = {},
): Promise<{ courses: CourseCatalogEntry[]; total: number }> {
  const { data, error } = await api.GET("/v1/courses/catalog", {
    params: {
      query: {
        q: params.q,
        max_price_minor: params.maxPriceMinor,
        page: params.page,
        page_size: params.pageSize,
        sort: params.sort,
      },
    },
  });
  if (!data) throw new Error(`Failed to load the course catalog: ${JSON.stringify(error)}`);
  return { courses: data.courses.map(toCatalogEntry), total: data.total };
}

/** A course's public landing page, unauthenticated (Server Component reads —
 *  `is_enrolled`/`is_owner` always read false; use `getMyCourseCatalogDetail`
 *  from a client component to personalise them). Returns null on a 404. */
export async function getCourseCatalogDetail(id: string): Promise<CourseCatalogDetail | null> {
  const { data, error, response } = await api.GET("/v1/courses/catalog/{id}", {
    params: { path: { id } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`Failed to load course "${id}": ${JSON.stringify(error)}`);
  return toCatalogDetail(data);
}

/** Same landing page, but through the browser client so a signed-in viewer's
 *  bearer token personalises `isEnrolled` / `isOwner`. Works logged out too. */
export async function getMyCourseCatalogDetail(id: string): Promise<CourseCatalogDetail | null> {
  const { data, error, response } = await browserApi.GET("/v1/courses/catalog/{id}", {
    params: { path: { id } },
  });
  if (response.status === 404) return null;
  if (error || !data) throw toErr(error, response.status, "Could not load that course.");
  return toCatalogDetail(data);
}

/** Buys (or free-enrolls in) a course. `methodToken` is ignored for a free
 *  course. Returns the enrollment plus whether this call created it fresh
 *  (201) vs. the caller was already enrolled / it's free (200). */
export async function purchaseCourse(
  id: string,
  methodToken?: string,
): Promise<{ enrollment: CourseEnrollment; created: boolean }> {
  const { data, error, response } = await browserApi.POST("/v1/courses/{id}/purchase", {
    params: { path: { id } },
    body: methodToken ? { method_token: methodToken } : {},
  });
  if (error || !data) throw toErr(error, response.status, "Could not complete the purchase.");
  return { enrollment: toEnrollment(data), created: response.status === 201 };
}

/** The caller's "my learning" list, newest first. */
export async function listEnrollments(): Promise<CourseEnrollment[]> {
  const { data, error, response } = await browserApi.GET("/v1/enrollments", {});
  if (error || !data) throw toErr(error, response.status, "Could not load your enrollments.");
  return data.enrollments.map(toEnrollment);
}

/** The enrolled-student (or owner-preview) curriculum player. */
export async function getCourseLearn(id: string): Promise<CourseLearnDetail> {
  const { data, error, response } = await browserApi.GET("/v1/courses/{id}/learn", {
    params: { path: { id } },
  });
  if (error || !data) throw toErr(error, response.status, "Could not load this course.");
  return toLearnDetail(data);
}

/** Records playback progress / completion on a video item. */
export async function recordCourseItemProgress(
  courseId: string,
  itemId: string,
  input: { positionSeconds?: number; completed?: boolean },
): Promise<CourseItemProgress> {
  const { data, error, response } = await browserApi.POST(
    "/v1/courses/{id}/items/{itemId}/progress",
    {
      params: { path: { id: courseId, itemId } },
      body: { position_seconds: input.positionSeconds, completed: input.completed },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not save your progress.");
  return toItemProgress(data);
}
