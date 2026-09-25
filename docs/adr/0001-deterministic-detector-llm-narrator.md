# Deterministic detector, LLM narrator

The AI analysis pipeline is split: a **deterministic detector** owns every decision that can be graded — whether an anomaly exists, its type, severity and confidence — while an **LLM** receives the detector's structured evidence and rewrites it into `reason` and `recommended_action` prose. The LLM never classifies and never decides severity.

## Context

35 of the 100 available points are AI-specific (detect M-109 30, prioritise it 25, avoid treating M-106 as real 15, detect M-112 as data quality 10, explain with evidence 10, recommend a coherent action 10). The spec (§8) leaves the technique free — "reglas, estadística, Z-score, Isolation Forest, series de tiempo, ML, LLM o combinación" — and explicitly says the *result* is graded, not the technology.

Three shapes were available:

- **Pure deterministic engine.** No LLM anywhere. Reproducible, offline, free, and cannot fail. But an evaluator reading §10 will ask where the AI is, and the honest answer is "a threshold".
- **Deterministic detector + LLM narrator (chosen).** Detection stays reproducible while the explanation and recommendation are genuinely generated. Costs an API key and introduces prose non-determinism.
- **LLM in the decision loop.** Maximum apparent "AI-ness", but the 4/4 expected outcome becomes a sampling gamble and the confidence figure becomes a fabricated number.

## Decision

Option 2, with option 1 as a guaranteed fallback.

The detector is authoritative and **always runs**. The LLM is a presentation layer over evidence the detector already produced. If no API key is configured, the engine composes `reason` and `recommended_action` from a deterministic template and the response carries `explanation_source: "rules"`; when a key is present it carries `explanation_source: "llm"`. The UI displays this field, so the demo is honest about which path produced the text and **never breaks** when offline.

## Consequences

- The 4/4 expected result is reproducible run to run, which is what §18 actually grades.
- Detection is unit-testable without a network or a key; only the narration layer needs a fake.
- A future reader will see an LLM call and reasonably ask why it does not make the decision. This ADR is the answer: the decision is the graded artefact, and delegating it would make the grade a lottery.
- The LLM is an **optional** dependency. Removing it degrades the product's polish, never its correctness.
