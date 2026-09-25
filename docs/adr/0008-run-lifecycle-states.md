# A run is COMPLETED when detection is done, not when narration is done

A run has three states, `RUNNING → COMPLETED | FAILED`. Narration progress is tracked **separately and per anomaly** as `explanation_status: PENDING | READY | FAILED`. Runs persist in an `analysis_runs` table holding `started_at`, `finished_at`, `state`, `anomaly_count` and the analysed window.

## Context

ADR-0006 made the analysis asynchronous with rules-first persistence. §5's dashboard needs "Último análisis — Fecha/hora y estado" to point at something real, and §13 requires visible process state.

## Consequences

**The name `COMPLETED` is a trap and must be documented as detection-complete.** Narration continues after it, and anything reading `COMPLETED` as "everything is finished" will be wrong. The API doc says so explicitly. The UI reads the two fields together — "Run complete · 4 anomalies · explaining 2/4" — which avoids modelling the awkward intermediate state where detection is done but narration is mid-flight, and avoids duplicating per-anomaly information on the run.

`analysis_runs` is additional to the PDF's suggested entities; it holds run history and gives the dashboard a real timestamp and state. There is no `consideraciones_persistencia.md` in this repository to update.

The latest attempt and the latest successful result need not be the same run. If a new attempt fails before detection, its `FAILED` state is shown explicitly while findings from the last completed run remain visible as **previous results**, never presented as findings of the failed attempt. Runs and findings remain attributable to their own run IDs.
