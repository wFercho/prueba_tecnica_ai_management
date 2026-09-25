# The dashboard's "Confianza IA" KPI is deliberately not a mean

The dashboard's confidence KPI is the **categorical band of the most urgent anomaly, always attributed to a meter** (`Alta (M-109)`), with the latest run's actual band distribution directly beneath it. The distribution is derived from current scores, never hardcoded to an illustrative example, and nothing is averaged into a bare number.

## Context

§5 lists a dashboard KPI: `Confianza IA — Métrica agregada`. ADR-0005 states that confidence scores are uncalibrated evidence scores, not comparable across detectors, and not probabilities — and warns *specifically* against averaging them into something that reads as likelihood. A plain mean could be read as a probability. The example in §11 gives `Media/Alta` for M-106, meaning either band is acceptable; it does not mandate a fixed distribution for every run.

## Consequences

This resolves §5 against ADR-0005 rather than violating it: the distribution *is* an aggregate metric, so §5 is satisfied, while the headline is never a naked number. It avoids the misleading reading "the system is 82% sure something is wrong" on a dashboard intended to communicate operational evidence.

Attributing the band to a meter also keeps the KPI actionable: `Alta (M-109)` says what to click, `0.82` says nothing.

The README records that §5's KPI is intentionally not a mean, so that nobody later "fixes" it into one. Depends on ADR-0005.
