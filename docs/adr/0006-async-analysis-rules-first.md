# Analysis is async, and the rules explanation is always persisted first

`POST /ai/analyze` returns `202` with a run id **after detection and rules-first persistence**. Every anomaly is persisted with a deterministic template explanation and `explanation_source: "rules"` before the response is sent; when a key is configured, LLM narration can upgrade each row in the background. Narration sits behind a single-method interface, `Narrate(ctx, evidence) (Narrative, error)`, with an OpenAI implementation and a rules implementation.

## Context

The suggested API lists both `POST /ai/analyze` and `GET /ai/analysis/:id`, and §13 requires showing process state. The detection stage finishes before the response, while **only optional LLM narration** continues asynchronously. ADR-0001 introduced a new failure mode the original API list predates: the LLM call can be slow, rate-limited, or entirely absent.

## Consequences

The §11 table is complete the instant the response arrives; no row ever renders with an empty explanation, because the template is always written first. The detector, the classification, and the four graded outcomes are durable **before anything touches the LLM network**, so an outage degrades the wording and nothing else. With no key, rows are `rules/READY`. With a key, they start `rules/PENDING`, then become `llm/READY` or retain the template as `rules/FAILED`; `PENDING` never means that the reason or action is blank.

A one-method interface for four anomalies is not over-engineering — it is what makes the narrator unit-testable with a fake, and it keeps the vendor out of the domain. The pipeline never blocks on the network.
