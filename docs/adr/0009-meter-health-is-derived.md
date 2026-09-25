# Meter health is derived; §6 shows health and anomaly severity separately

A meter's health is a **derived** value in `{HEALTHY | ALERT | CRITICAL}`, computed from the greatest operational impact of its open anomalies after sufficient analysis. Before analysis, or with insufficient history for a meter, the UI shows `Sin analizar`/`Sin datos`, not `HEALTHY`. The meter list also shows anomaly severity in a separate column, as the original PDF's §6 does.

## Context

The original PDF's §6 labels its last column `Anomalía`, not `Nivel` as the Markdown transcription does. It pairs `Alert` with `High` for M-112 and `Critical` with `High` for M-109, so health cannot be a projection of severity alone. For the delivered dataset, M-109 is `CRITICAL`, M-104 and M-112 are `ALERT`, and M-106 is `HEALTHY`: the scheduled outage is a `FALSE_POSITIVE` and should not be escalated. The severity column remains visible, even when the meter's health is not `CRITICAL`.

## Consequences

`HEALTHY / ALERT / CRITICAL` describes a **meter**; `HIGH / MEDIUM / LOW` describes an **anomaly**. They are not interchangeable. The dashboard's high-priority count uses `HIGH` severity, including the M-112 data-quality anomaly, not `CRITICAL` health.

Health is recomputed on read and never persisted. Storing it on `meters` would let the seed, the detector and the UI each hold a different opinion about the same meter, with no single owner of the truth.

Keeping both columns makes the PDF's counterexample legible: M-112 has `ALERT` health and a `HIGH`-severity anomaly without falsely implying it is a critical consumption anomaly. If the same meter also has a `REAL_ANOMALY/HIGH` episode, that episode raises its health to `CRITICAL`.
