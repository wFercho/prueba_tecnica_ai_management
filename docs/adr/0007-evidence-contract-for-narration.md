# Narration receives the episode's own readings, not just aggregates

The `Narrate` boundary carries a `NarrateEvidence` struct containing meter identity, the episode window, baseline vs actual for that window, the **per-hour deviation series inside the episode only** (consumption, voltage, current, power factor, baseline and deviation), context reports including explicit absence of an explanatory event, the four `confidence_basis` terms, and separate data-quality findings with their hours and offending values. The same evidence must cross the actual provider request boundary; populating a Go struct but omitting values from the LLM payload would defeat the decision. It is bounded: no full history.

## Context

ADR-0001 made the LLM the narrator, which left open *what crosses the boundary* — and the cheap answer is a summary of aggregates. But a summary means the LLM can only restate what the rules engine already computed. That is a template with extra steps, and it hollows out ADR-0001: if the prose is derivable from the inputs, the LLM contributed nothing and the fallback path is the real product.

## Consequences

M-109's episode is 58 hourly points — small enough to send whole, and exactly what lets the LLM say **when** the change happened rather than only that it happened. That is the substance of §12's "Evidencia que soporta la explicación" and §18's 10 points for explaining with evidence.

Sending the meter's full history was rejected: the baseline already encodes normal behaviour, so those readings carry no information the summary lacks. The cap keeps the payload bounded even if a future episode runs long.
