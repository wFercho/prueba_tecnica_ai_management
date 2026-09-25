# React + TanStack + Tailwind for a Spanish-first demo

The PDF requires an effective frontend with navigation, filters and graphs, but does **not** prescribe libraries. We choose to keep the existing React, TypeScript and Vite app and add Tailwind CSS 4 for styling, TanStack Router for route-level navigation and guarded deep links, TanStack Query for server state/mutations/polling, TanStack Table for filtering and sorting, and Recharts as the charting first choice (subject to the bounded M-109 spike in ADR-0012). All operator-facing UI text, rules explanations and LLM narration must be in Spanish; machine identifiers and ingested source values remain unchanged.

## Context

The current app uses local component state as navigation, manual fetch/timeouts, CSS and SVG. Those choices kept the first implementation small, but the agreed product adds login, direct navigation to specific meters/anomalies, live per-row narration, sortable/filterable tables and an evidence-rich chart. Keeping several ad hoc versions of those mechanisms as the app grows would make the demo harder to test and navigate. This is a **project choice for those interactions**, not a claim that the PDF requires the selected packages.

## Consequences

- Replace the local-view switch with actual routes, while keeping the Go API as the authentication authority; the frontend guard alone is not security.
- Use Query for remote data and stop polling when narration settles; keep local presentation state local. Use Table for the two data tables, not for the detector's ordering rules.
- Tailwind provides styling primitives without forbidding focused component CSS or an accessible component library if needed. The chart must still meet the four visual gates and keyboard/text alternatives regardless of Recharts.
- Translate visible labels, actions, validation, errors, legends, tooltips, dates and units; translate the meaning of delivered English context reports for display without rewriting the source data. Reject or fall back from LLM prose that violates the Spanish narration contract.
- Do not introduce TanStack Start while Go serves the backend, TanStack Form for a single login form, or TanStack Virtual for twelve meters without a measured need.
