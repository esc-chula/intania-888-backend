# Frontend handoff: admin-managed sport types

The backend adds sport management under `/api/v1/sport-types/admin` and a single
catalogue-entry read. Catalogue resources remain `{ id, title }`. This document
describes the separate frontend implementation and its acceptance checks.

## API operations

| Operation | Route | Response |
| --- | --- | --- |
| Load catalogue | `GET /sport-types` | `200` resource array |
| Load one entry | `GET /sport-types/{id}` | `200` resource |
| Create (admin) | `POST /sport-types/admin` with `{ id, title }` | `201` resource |
| Rename (admin) | `PATCH /sport-types/admin/{id}` with `{ title }` | `200` resource |
| Delete (admin) | `DELETE /sport-types/admin/{id}` | `204`, no response body |

Reads require an authenticated browser session. Mutations also require admin
permission, allowed Origin, and the session CSRF token. Use the existing shared
API client; encode IDs as path segments and preserve the shared error envelope.

IDs are immutable, nonempty, and at most 100 ASCII characters using letters,
digits, underscores, or hyphens. Create and rename trim titles and require 1–100
characters. Count Unicode characters for client validation, rather than UTF-16
code units. Duplicate titles are allowed. Unknown fields are rejected; PATCH
must not send `id`. Do not derive IDs from edited titles.

| Failure | UI handling |
| --- | --- |
| `400 INVALID_REQUEST` | Show field details and preserve input. |
| `409 CONFLICT` on create | Show that the ID already exists; allow choosing another. |
| `404 RESOURCE_NOT_FOUND` | Refresh the catalogue; explain the entry is no longer available. |
| `409 SPORT_TYPE_IN_USE` on delete | Explain that matches, tournament groups, or stages still use the sport; keep it visible. |
| `401`, `403`, dependency or unexpected failure | Use shared session, permission, retry, and request-ID handling. |

## Current frontend consumers

Reference: sibling `intania-888-frontend` at `105fd51`, inspected on 2026-09-29.
Recheck these consumers if its source changes before implementation.

| File / area | Required change |
| --- | --- |
| `src/api/sportType.ts` | Add single-read, create, rename, and delete functions using the admin mutation paths. Keep the existing catalogue type. |
| `src/app/admin/sports/page.tsx` | Add create, rename, and delete controls; replace the read-only notice. Keep immutable IDs visible. Remove the obsolete backend constants reference from the product UI. |
| `src/components/match/MatchMapAndList.tsx` | Replace fixed sport titles, choices, and title-to-ID reverse maps with API entries. Preserve unrelated color/league presentation choices. |
| `src/app/page.tsx`, `src/app/match/page.tsx`, `MatchBanner.tsx`, `SlipElement.tsx`, and `SlipResult.tsx` | Replace fixed choices/reverse maps; resolve current titles by ID from the shared catalogue and display unknown IDs as a fallback. Update the shared `Selector` to accept distinct ID/value and label pairs. |
| Admin match create/edit/list pages | They already load the catalogue; ensure they share refreshed data after sport mutations and use IDs for selections and React keys. |

## Shared catalogue behavior

Load the catalogue through one shared hook or store with loading, empty, and error
states. Refresh or invalidate it after every successful sport mutation. Refetch
when entering other sport-consuming screens so changes made in another browser
session become visible.

Use `sport.id` for keys, option values, and lookup-map keys. Display `sport.title`
as the label. Two sports with the same title must remain separately selectable;
include their IDs when needed to distinguish them. Never reverse-map a title to
an ID: duplicate seeded titles already exist.

Keep an “all sports” filter as separate UI state, such as `null`, rather than a
catalogue ID named `ALL`. `ALL` is itself a valid sport ID under the API rules.
Custom sports must receive a usable default presentation without requiring a
new hardcoded category or style entry. Renaming updates labels while retaining
selection IDs. If a selected unused sport is deleted, clear that stale selection
when the catalogue refreshes.

Delete only removes unused entries. There is no archive control in this release.
Catalogue reseeding preserves renamed titles, but explicitly rerunning the seed
can restore a deleted default entry; do not treat a restored entry as a client bug.

## Client acceptance checks

- Type-check the implementation with `pnpm exec tsc --noEmit --incremental false`.
- As admin, create a custom sport and check its visibility in admin management,
  match create/edit selectors, user filters, and labels after refresh.
- Rename it and confirm the ID is unchanged, current labels refresh, and existing
  selections remain attached to that ID.
- Create two sports with the same title; confirm each ID remains selectable.
- Reject empty/invalid/overlong IDs and empty/overlong trimmed titles; show backend
  validation and duplicate-ID failures without losing the form.
- Delete an unused entry and remove it from refreshed selectors. Treat `204` as
  success without decoding JSON.
- Attempt to delete a referenced entry; display `SPORT_TYPE_IN_USE`, preserve the
  entry, and confirm existing fixtures/groups remain visible.
- Verify regular users can load the catalogue but cannot mutate it. Check missing
  session, admin permission, Origin, and CSRF failures through shared handling.
- Verify catalogue loading/empty/error states, a custom ID named `ALL`, duplicate
  display names, and unknown-ID label fallbacks.

Backend verification does not establish these frontend acceptance results; run
them when the client implementation is complete.
