/**
 * Course-authoring data access (browser). Wraps the generated client, maps
 * wire ↔ view-model, and throws a typed CourseError on failure. Mirrors
 * `features/resources/api.ts`'s conventions.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type { Money } from "@/types/teacher";
import type {
  Course,
  CourseDetail,
  CourseItem,
  CourseItemKind,
  CourseSection,
  CourseStatus,
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
  priceCurrency?: Money["currency"];
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
    priceCurrency: Money["currency"];
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
  input: { kind: CourseItemKind; title?: string; videoAssetId?: string; resourceId?: string },
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
      },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not add that item.");
  return toCourseDetail(data);
}

export async function renameItem(
  courseId: string,
  sectionId: string,
  itemId: string,
  title: string,
): Promise<CourseDetail> {
  const { data, error, response } = await browserApi.PATCH(
    "/v1/courses/{id}/sections/{sectionId}/items/{itemId}",
    {
      params: { path: { id: courseId, sectionId, itemId } },
      body: { title },
    },
  );
  if (error || !data) throw toErr(error, response.status, "Could not rename that item.");
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
