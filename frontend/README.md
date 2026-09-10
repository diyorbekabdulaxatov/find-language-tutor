# FindTutor — frontend

Next.js (App Router) frontend for the FindTutor language-tutoring marketplace.
Market: Uzbekistan. Prices in UZS, UI in English.

## Stack

| Concern | Choice |
| --- | --- |
| Framework | Next.js 16 (App Router), React 19 |
| Language | TypeScript (strict) |
| Styling | Tailwind CSS v4 (CSS-based config in `src/app/globals.css`) |
| Components | shadcn/ui (radix-nova style) in `src/components/ui` |
| Fonts | Fraunces (display) + Archivo (text), self-hosted via `next/font` |
| Data | mock async API (`src/features/teachers/`) until the real API exists |

## Run

```bash
npm install
npm run dev      # http://localhost:3000
npm run build    # production build + typecheck
npm run lint
```

## Structure

```
src/
  app/                     routes (App Router)
    page.tsx               home
    teachers/
      page.tsx             listing — reads ?lang &kind &max &sort (dynamic)
      loading.tsx          skeleton shown while the server component streams
      [slug]/page.tsx      profile — prerendered via generateStaticParams (SSG)
  components/
    ui/                    shadcn primitives (owned, editable)
    layout/                site header + footer
  features/
    teachers/              domain module — mirrors a backend module
      api.ts               data-access fns; signatures match the future openapi-fetch client
      mock-data.ts         seed teachers (delete once the API lands)
      components/          teacher-specific UI
  lib/                     format.ts, i18n.ts, utils.ts
  types/                   hand-written domain types (replaced by generated ones later)
```

## How data flows

Server Components call `features/teachers/api.ts` directly (no fetch, no hooks).
When `openapi.yaml` exists, only the bodies of those functions change — they'll
call the generated `openapi-fetch` client. Callers stay the same.

## Notes

- Prices are stored in **minor units** (`amountMinor`) like Stripe. `formatMoney`
  renders `{ 9_000_000, "UZS" }` as `90,000 so'm`.
- Teacher availability, booking, auth, and payments are not built yet — buttons
  for those are present but inert.
- Next.js 16 ships its own docs in `node_modules/next/dist/docs/`; check there
  before assuming an API matches older tutorials (`params`/`searchParams` are
  Promises, route prop types come from `PageProps<'/route'>`).
