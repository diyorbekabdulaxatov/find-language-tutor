import createClient from "openapi-fetch";
import { getLocale } from "next-intl/server";
import type { components, paths } from "./schema";

/**
 * Typed client for the Go backend, generated from ../openapi.yaml
 * (`npm run gen:api` regenerates the schema).
 *
 * `API_URL` has no NEXT_PUBLIC_ prefix on purpose: every call today runs in a
 * server component, so the browser never needs the backend origin. Add a
 * separate public base URL only when something fetches from the client.
 */
const baseUrl = process.env.API_URL ?? "http://localhost:8080";

export const api = createClient<paths>({ baseUrl });

// Server components fetch on the viewer's behalf: pass their UI locale on so
// any error message the backend returns is already in their language.
api.use({
  async onRequest({ request }) {
    request.headers.set("Accept-Language", await getLocale());
    return request;
  },
});

export type ApiSchemas = components["schemas"];
