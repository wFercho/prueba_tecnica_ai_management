# Anomalies are episodes, and false positives are rows

An anomaly is **one related episode on one meter**, stored as a single row — not one row per reading. An episode may contain unaffected readings between affected ones. The M-106 scheduled outage is **persisted as a `FALSE_POSITIVE` row**, not suppressed at detection.

## Context

§11 shows exactly one result row per affected meter and §13 promises "4 anomalías detectadas". But the episodes are large: M-109 has 58 anomalous hours, M-106 has 12 outage hours, M-112 has 16 corrupt rows separated by normal hourly readings. Per-reading rows would produce many anomalies instead of 4.

Whether to *store* a false positive was a genuine fork. §18 awards 15 points for "Evita tratar M-106 como anomalía real", and §11 displays M-106 in the results table with the action "No escalar" — so the spec shows it, not hides it.

## Decision

One `anomalies` row per meter per related episode, carrying `window_start`, `window_end`, `affected_reading_count` and `detected_by`. The count includes only affected readings, not normal readings inside the window. `type` is a fixed four-value enum: `REAL_ANOMALY | DATA_QUALITY | EXPLAINABLE | FALSE_POSITIVE`. Where the LLM and the detector disagree, the detector always prevails (see ADR-0001).

Intermittent findings of the same type can belong to one episode when successive affected readings are at most three hours apart (configurable); a later finding after a longer gap starts a new episode. M-112 is sampled hourly, but its defective readings recur every three hours — that is not a three-hour sampling cadence.

## Consequences

The dashboard's "anomalías detectadas" count of 4 and §11's table both stay coherent. Storing the false positive is what actually demonstrates the §18 behaviour: the system *examined* the outage, found a 36% drop, correlated it to the scheduled maintenance event, and deliberately downgraded it — rather than never having seen it. Suppressing it would leave an evaluator unable to tell those two cases apart.

The cost is that "detected" must be defined as *episodes found, including downgraded ones*. The dashboard reports 4 found and 2 needing priority; any count that silently drops the false positive would misrepresent what the detector did.
