# Phase 02 API Coverage

No external API integration: Phase 2 is SPA-only tree selection; the detected "wrapping + api" clause (02-01-PLAN.md line 130) is the vi.mock test harness for the app's internal same-origin /api client, not an external surface.

## Rationale

The seal-time detector fired on a single clause in `02-01-PLAN.md`'s context-file
list: "vi.mock api harness + import-after-mock + provider wrapping". The `+`
separators are not clause boundaries to the detector, so the integration verb
("wrapping" — React provider wrapping in the test harness) paired with the API
noun ("api" — the app's own first-party client, `web/src/lib/api.ts`, called
same-origin `/api` only, per the project constraint "SPA: same-origin `/api`
only"). Phase 2 touched `web/` exclusively; no third-party API, SDK, or service
was introduced.
