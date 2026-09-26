# Seed restarts the analysis demo, not the user database

The evaluator must see the first analysis happen in the app. A repeated `make seed` therefore replaces the delivered meters, readings and context reports and removes the dependent anomalies and run history, leaving no precomputed findings. This is an explicit demo reset, not an upsert that retains stale analysis of earlier source data.

## Context

Keeping old findings after reimport makes "Último análisis" and the four-result table look precooked or potentially inconsistent with the new readings. Resetting the whole database would also destroy users and active sessions, making the demo login unreliable. The seed owns the input dataset and its derived analysis only; identities have a separate lifecycle (ADR-0013).

## Consequences

`make up` starts the app and provisions the initial demo user; `make seed` clears analysis outcomes and imports source rows, preserving users and sessions. The evaluator then signs in and presses «Ejecutar análisis IA» in the UI. A second analysis click without reseeding creates a new run and retains history, while the UI shows findings from the latest successful run and warns if a newer attempt failed.
