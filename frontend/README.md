# FindTutor — frontend

Next.js (App Router) frontend for the FindTutor language-tutoring marketplace.
Market: Uzbekistan. Prices in UZS; UI in English, Russian and Uzbek.

## Stack

| Concern | Choice |
| --- | --- |
| Framework | Next.js 16 (App Router), React 19 |
| Language | TypeScript (strict) |
| Styling | Tailwind CSS v4 (CSS-based config in `src/app/globals.css`) |
| Components | shadcn/ui (radix-nova style) in `src/components/ui` |
| Fonts | Bricolage Grotesque (display) + Plus Jakarta Sans (body), self-hosted via `next/font` |
| Data | `openapi-fetch` client generated from `../openapi.yaml` (`npm run gen:api`) |
| i18n | `next-intl`, cookie-based locale, typed message keys (`messages/{en,ru,uz}.json`) |
| Tests | Vitest for pure logic + catalogue checks (`npm test`) |

## Run

```bash
npm install
npm run dev      # http://localhost:3000
npm run build    # production build + typecheck
npm run lint
npm test         # vitest
npm run gen:api  # regenerate src/lib/api/schema.ts from ../openapi.yaml
```

## Structure

```
messages/                  en.json / ru.json / uz.json — one namespace per feature
src/
  app/                     routes (App Router): /, /teachers, /courses, /learn,
                           /bookings, /dashboard, /resources, /grading, /admin/*, auth pages
  components/
    ui/                    shadcn primitives (owned, editable)
    layout/                site header + footer, theme + locale switchers
  features/<module>/       mirrors a backend module: api.ts (wire → view-model),
                           types.ts, components/   (auth, teachers, availability,
                           bookings, reviews, dashboard, resources, submissions,
                           courses, admin)
  i18n/                    locale config, next-intl request config, setLocale action
  lib/                     api client + generated schema, format.ts, i18n.ts, country.ts
  types/                   hand-written camelCase view-models
```

## How data flows

Server Components call `@/lib/api/client` (no credentials; forwards the UI
locale as `Accept-Language`). Browser code calls `@/features/auth/browser-client`,
which attaches the in-memory access token, sends the refresh cookie, and does
exactly one refresh-and-retry on a 401. Each feature's `api.ts` maps the
snake_case wire types to camelCase view-models; components never see wire
shapes.

## Notes

- Prices are stored in **minor units** (`amountMinor`) like Stripe. `formatMoney`
  renders `{ 9_000_000, "UZS" }` as `90,000 so'm`.
- The locale is a cookie (`NEXT_LOCALE` → `Accept-Language` → `en`), not a URL
  segment, so every route is dynamic. A missing message key is a compile error.
- Next.js 16 ships its own docs in `node_modules/next/dist/docs/`; check there
  before assuming an API matches older tutorials (`params`/`searchParams` are
  Promises, route prop types come from `PageProps<'/route'>`).
