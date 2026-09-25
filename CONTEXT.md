# AI Energy Management

Detects, explains, prioritises and recommends actions on electrical anomalies across a fleet of meters. The value is not that anomalies are flagged — it is that a person can see in seconds which one needs attention first and understand why.

## Language

### Catalog

**Meter**:
An electrical meter under monitoring, identified by a stable human-readable code such as `M-101`.
_Avoid_: device, sensor, equipment, point

**Reading**:
One hour of measured values for one meter: consumption, voltage, current and power factor. The delivered dataset is a complete 12 × 336 hourly grid with no gaps.
_Avoid_: sample, measurement, datapoint, observation

**Ingested status**:
The `status` value the source supplied alongside a reading, preserved verbatim. It reads `OK` on all 4,032 delivered rows — including the corrupt ones — which makes it evidence about the ingest pipeline, never a verdict about the reading.
_Avoid_: quality flag, verdict, validity

**Operational event**:
A known change in a meter's operating conditions, such as a production line starting or a scheduled outage, that can explain a consumption episode.
_Avoid_: no-event report, data-quality report, anomaly

**Context report**:
An entry supplied with the readings that describes an operational event, a reported data fault or an explicit lack of a known event. A report is evidence to assess, not an automatic classification.
_Avoid_: anomaly, detector verdict

**No-event report**:
A context report that says no known operational event accounts for a meter's change. It is evidence of an unexplained change, not an event that explains it.
_Avoid_: operational event, outage

### Analysis

**Baseline**:
The expected consumption for one meter at one hour of day, over a trailing window that excludes the candidate episode. Learned per meter, never global.
_Avoid_: average, mean, expected value, norm

**Episode**:
A related sequence of anomalous readings on one meter, bounded by its first and last affected reading. An intermittent episode may include normal readings between affected readings; the affected-reading count excludes them.
_Avoid_: event, window, incident, span

**Anomaly**:
One classified episode with a type, severity, confidence and recommended action. A meter under two separate episodes has two anomalies.
_Avoid_: alert, issue, finding, problem, detection

**Recommended action**:
A proposed next step for investigating an anomaly, not a command executed by the platform or a record that the step has been performed.
_Avoid_: action taken, resolution, disposition

**Anomaly type**:
One of `REAL_ANOMALY`, `DATA_QUALITY`, `EXPLAINABLE`, `FALSE_POSITIVE`. A fixed vocabulary, not an open label.
_Avoid_: category, label, class, kind

**Severity**:
How much attention an anomaly warrants, judged independently of how confident the detector is. A high-severity finding can carry low confidence and must still be shown.
_Avoid_: priority, risk, urgency

**High-priority anomaly**:
An anomaly with `HIGH` severity, whether its type is real consumption change or data quality; `MEDIUM` and `LOW` do not count as high priority.
_Avoid_: needs attention, high confidence, critical meter

**Confidence**:
The detector's reproducible evidence score in [0, 1] for its assigned anomaly type, including `FALSE_POSITIVE`, built from four evidence terms. It is **not a probability** or a measure of operational urgency.
_Avoid_: probability, likelihood, certainty, trust

**Evidence term**:
One of the four kinds of evidence behind confidence: `deviation`, `event_match`, `corroboration`, `persistence`.
_Avoid_: factor, feature, signal, weight

**Quality finding**:
A specific observation supporting a data-quality episode, such as a physically inconsistent electrical value or repeated abnormal readings, with the offending values and time distinguished from the score's four evidence terms.
_Avoid_: confidence basis, source status, anomaly type

**Insufficient history**:
A meter lacks enough trustworthy readings to establish its own baseline. It is not evidence of healthy operation and is not itself a consumption anomaly.
_Avoid_: healthy, zero anomalies, normal

**Health**:
A meter's operational standing after sufficient analysis, `HEALTHY`, `ALERT` or `CRITICAL`, determined by the greatest operational impact of its open anomalies. Before analysis or with insufficient history, health is unknown rather than healthy. An isolated high-severity data-quality episode warrants `ALERT`; a high-severity real anomaly warrants `CRITICAL` even if a data-quality episode is also open.
_Avoid_: status, level, state, Nivel, OK, Alert, Critical

### Workflow

**Run**:
One analysis of a dataset. Its completion means detection is finished, not necessarily narration of every anomaly; narration has a separate state.
_Avoid_: job, task, execution, batch

**Anomaly status**:
Whether an anomaly is awaiting human review. `OPEN` is the only supported status in the current product; a recommended action does not imply that anyone performed it.
_Avoid_: state, stage, disposition

**Explanation source**:
Whether an anomaly's explanation was composed by deterministic rules or by an LLM: `rules` or `llm`.
_Avoid_: origin, provider, author, generated_by

**Explanation status**:
Whether optional LLM narration is pending, ready or failed: `PENDING`, `READY` or `FAILED`. Distinct from anomaly review status and explanation source; a failed upgrade retains the existing rules explanation.
_Avoid_: status, state, progress
