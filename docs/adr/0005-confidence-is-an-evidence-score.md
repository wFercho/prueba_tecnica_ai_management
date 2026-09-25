# Confidence is an uncalibrated evidence score, not a probability

The detector computes `confidence` as a weighted product of four evidence terms — `deviation`, `event_match`, `corroboration`, `persistence` — and the API returns those terms as `confidence_basis`. It scores the evidence for the **assigned type**, including `FALSE_POSITIVE`, not the chance that a real consumption anomaly exists. The field is named `confidence` to match §10's payload but is **documented as an uncalibrated score in [0,1], not a probability**. The dashboard and the §11 table show a categorical band; the detail view shows the number alongside its derivation.

## Context

The spec contradicts itself. §10's JSON payload shows `"confidence": 0.96` — numeric, formatted as a probability. §11's own results table shows `Confianza: Alta` and `Media/Alta` — categorical, for the same field.

A probability is the stronger claim and cannot be honestly made here. A language model cannot self-report a calibrated probability, and a threshold on a z-score cannot either: nothing in the dataset is calibrated against known truth, because `expected_results.csv` is reserved for the evaluator. A bare `0.96` is therefore the single place this system could be dishonest, and the most likely place an evaluator will poke.

## Consequences

- The LLM must never emit this field, under any circumstance (ADR-0001). Prose is generated; the score is measured.
- Returning the four terms makes the number traceable to the specific readings that produced it, and satisfies §12's "Evidencia que soporta la explicación" with the same evidence rather than a second, parallel story.
- The categorical band in §11 is derived *from* the score, so there is one number and one formula, not two independent judgements.
- Anyone consuming the API must be told plainly that these scores are not comparable across detectors and are not probabilities, so nobody later averages them into a ranking and presents it as likelihood.
