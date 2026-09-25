# Energy anomaly management

Detects anomalous consumption in hourly meter readings, classifies what it found, and
lets an operator work through the result in a dashboard.

The detector is deterministic and rule-based. An LLM is optional and only rewrites the
explanation of an anomaly that the rules already found — it can never create, reclassify
or discard a finding. The MVP runs with no API key at all.

## Quick start

Everything runs in Docker. A clean checkout needs only Docker and `make`.

```sh
make up        # build the image, start PostgreSQL and the API, wait until both are healthy
make seed      # import the delivered readings and events
make analyze   # run the detector over the imported data
```

Then open <http://localhost:8090>.

The dashboard shows four anomalies across twelve meters, and the other eight meters stay
silent:

| Meter | Type | Severity | Confidence | What happened |
| --- | --- | --- | --- | --- |
| M-104 | `EXPLAINABLE` | MEDIUM | 0.79 | +46.6% over 96 hours, matching a reported operational change |
| M-106 | `FALSE_POSITIVE` | LOW | 0.83 | −79.8% over 12 hours, matching a scheduled outage |
| M-109 | `REAL_ANOMALY` | HIGH | 0.99 | +110% over 58 hours with voltage and current moving too, and no event to explain it |
| M-112 | `DATA_QUALITY` | HIGH | 0.96 | 16 hours of physically implausible readings |

`make help` lists every target. `make down` stops the stack and keeps the data;
`make fresh` deletes the database volume.

## How it is put together

```
frontend/            React 19 + Vite dashboard, TanStack Router/Query, Tailwind CSS 4
backend/
  cmd/server          the API; also serves the built dashboard
  cmd/seed            the one-shot import of the delivered CSVs
  internal/analysis   the detector: baselines, episodes, classification, confidence
  internal/store      the persistence contract, its in-memory and PostgreSQL implementations
  internal/service    run lifecycle, narration, dashboard and meter queries
  internal/httpapi    HTTP, one file per concern
  internal/narrate    the narrator contract, the rules narrator, the OpenAI narrator
data/                 the delivered readings.csv and events.csv
docs/adr/             why it is built this way
```

The dependency direction is one-way: `httpapi` → `service` → `analysis`, with `store`
and `catalog` underneath and `narrate` beside it. Nothing above the store knows SQL, and
nothing above the service knows HTTP. That is what lets the whole API be tested without a
database and the detector be tested without either.

### Persistence

PostgreSQL 17 through `pgx`, without an extension. The schema is two embedded SQL files
applied on every boot, each idempotent, so a second `make up` is not a special case.
Tables: `meters`, `readings`, `events`, `analysis_runs`, `anomalies`. Reads are
hourly-series queries; the dashboard's total consumption is one aggregate rather than
4,032 rows.

An anomaly is stored as an **episode**: the hours it covers, the evidence behind it, the
per-hour deviation series computed against the baseline that judged it, the event that
corroborates or fails to explain it, and the prose. The deviation series is persisted
rather than recomputed on read, because the baseline is derived state and a re-read would
show a different series than the one the detector acted on.

## The API

All routes are served on one port. The dashboard is static, so the API and the UI share
an origin and there is no CORS anywhere.

The panel is at `/`, and a meter can be opened directly at `/medidores/M-109`.
The page uses client-side navigation; API requests remain on `/meters/...` and
continue to return JSON, including `/meters/M-109/readings`.

| Method | Route | What it does |
| --- | --- | --- |
| `GET` | `/meters` | The catalogue |
| `GET` | `/meters/{meterId}` | One meter: health, its anomalies, its events, and its series against its baseline |
| `GET` | `/meters/{meterId}/readings` | The raw hourly series, windowed |
| `GET` | `/anomalies` | The latest run's anomalies, most urgent first |
| `GET` | `/anomalies/{id}` | One anomaly with its evidence |
| `PATCH` | `/anomalies/{id}` | Moves it to `ACKNOWLEDGED`, `RESOLVED` or `DISMISSED` |
| `POST` | `/ai/analyze` | Runs the detector, returns `202` with the run |
| `GET` | `/ai/analysis/{id}` | Progress of one run |
| `GET` | `/dashboard/summary` | What the dashboard needs in one request |

```sh
curl -X POST localhost:8090/ai/analyze
# {"run":{"id":1,"state":"COMPLETED","anomaly_count":4,...}}

curl localhost:8090/dashboard/summary | jq
curl localhost:8090/anomalies | jq '.anomalies[] | {meter_id, type, severity, confidence}'
curl localhost:8090/meters/M-109 | jq '.points[13:15]'
```

`POST /ai/analyze` returns as soon as the findings are persisted, and the narration of
each finding continues in the background. The client polls
`GET /ai/analysis/{id}` until `narrating` reaches zero; each anomaly reports
`explanation_source` (`rules` or `llm`) and `explanation_status`
(`PENDING`, `READY`, `FAILED`).

## What the detector does

A baseline is an hourly profile — the frozen shape of what a meter's consumption looks
like at each hour of the day, learned from that meter's own history, never from other
meters. Readings outside the window are excluded from it, so an anomaly cannot explain
itself away.

An episode is a run of consecutive hours that deviate from that baseline, and it becomes
one finding rather than 96. The four types are decided in this order:

- **`DATA_QUALITY`** — the readings themselves are implausible: a power factor outside
  `0.80..1.00`, a voltage outside `200..240 V`, or energy that does not reconcile with
  voltage, current and power factor. The reading is rejected rather than believed.
- **`FALSE_POSITIVE`** — the deviation matches a reported operational event such as a
  scheduled outage.
- **`EXPLAINABLE`** — the deviation matches a reported event that accounts for it.
- **`REAL_ANOMALY`** — a real deviation with no event to account for it, ideally
  corroborated by a second measurement moving the same way.

Severity is judged independently of the classification: how much attention the finding
deserves, given its magnitude, its persistence and its corroboration.

**Confidence is an evidence score, not a probability.** It is a weighted geometric mean
of four terms — deviation `0.40`, event match `0.20`, corroboration `0.25`, persistence
`0.15` — and it is never presented as a percentage of being right. It is read
categorically as `HIGH`, `MEDIUM` or `LOW`, and the dashboard orders by it.

Meter health is derived, never stored: `CRITICAL` with a high-severity finding, `ALERT`
with a medium one, `HEALTHY` otherwise.

Every threshold lives in `analysis.DetectorConfig` and is calibrated against the
delivered control set, which the test suite pins.

## Narration

The rules narrator writes the prose and is always available. With `OPENAI_API_KEY` set,
the OpenAI narrator is offered each episode's evidence and may rewrite `reason` and
`recommended_action` in clearer language.

It is given the episode's numbers, the events in its window, and the classification the
rules already made, and it is checked on the way back: the classification, severity and
confidence must be unchanged, and the prose must be prose and not an instruction. A reply
that fails any of those checks is discarded and the rules prose stands. A narrator that is
unreachable, slow or unhelpful costs the system its explanation quality and nothing else —
the finding is already persisted.

## Configuration

Every setting has a default, so the stack runs with no `.env` at all. The
backend reads `DATABASE_URL`, `HOST`, `PORT`, `DATA_DIR`, `STATIC_DIR` and the
narration settings; Docker Compose configures its published ports separately.

| Variable | Default | Meaning |
| --- | --- | --- |
| `DATABASE_URL` | Compose injects `postgres://energy:energy@db:5432/energy?sslmode=disable`; direct Go runs default to `postgres://postgres:postgres@localhost:5432/energy?sslmode=disable` | PostgreSQL connection |
| `HOST` | *(empty, all container interfaces)* | Interface the API listens on inside its container (or on the host when run directly) |
| `PORT` | `8080` | The port the API listens on inside its container |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `OPENAI_API_KEY` | *(empty)* | Enables the OpenAI narrator. Empty means rules-only |
| `OPENAI_MODEL` | `gpt-4o-mini` | Which model narrates |
| `OPENAI_BASE_URL` | *(empty)* | Optional endpoint override for narration |
| `DATA_DIR` | `data/` | Directory containing `readings.csv` and `events.csv` |
| `STATIC_DIR` | `frontend/dist/` | Built dashboard served by the API |
| `BIND_HOST` | `127.0.0.1` | Host interface where Compose publishes **both** API and PostgreSQL |
| `API_PORT` | `8090` | Host port where Compose publishes the API |
| `DB_PORT` | `55433` | Host port where Compose publishes PostgreSQL |
| `DB_PASSWORD` | `energy` | Demo-only PostgreSQL password used by Compose |
| `API_URL` | `http://localhost:${API_PORT:-8090}` | Vite development proxy target; a full URL overrides the default host and port |

`OPENAI_API_KEY` is read from the environment and never logged; connection strings are
redacted before they reach a log line. Compose reads its variables from the host
environment (or an ignored local `.env`); it forwards `OPENAI_BASE_URL` to the
backend too. No secret needs to be committed. The
database password and any demo account credentials are **only for local use**.
Both ports bind to loopback by default; do not set `BIND_HOST=0.0.0.0` for
public access with demo credentials. `HOST` controls where the API listens
*inside* its container (or when run directly), not which host interfaces Docker
publishes. For local backend-only work, `HOST=127.0.0.1` restricts the Go
listener to loopback.

The frontend calls relative paths, so the built dashboard needs no API URL at
runtime. When using `pnpm dev`, set `API_URL` for a remote API target or
`API_PORT` to match a nondefault Compose port; see `frontend/README.md`.

## Tests

```sh
make test        # everything, in a container, against a throwaway database
make test-sql    # only the SQL
make vet
make smoke       # walk the running API and check every field the dashboard reads
```

The suite is organised by the seam it protects. The detector's tests pin the delivered
dataset to the four expected findings and the eight silent meters. The service and HTTP
tests pin the run lifecycle: findings are persisted before the caller is answered,
narration continues in the background, a failed narration leaves the rules prose in place
and marks itself failed, and a detector failure is recorded on the run rather than
swallowed. The store tests pin the contract in Go, and `internal/store/postgres` pins the
same contract in SQL against a real database — which is where the empty-history bug and
the redaction bug were found, both invisible to the in-memory tests.

`make smoke` is the one that speaks for the browser: it drives the live API and asserts
that every field the dashboard reads exists, is the right type, and carries the value the
UI claims. It is where the fourth health value was caught — the backend said `ALERT`, the
frontend said `WARNING`, and neither had noticed because nothing had asked both.

## Working on it

The Docker path is the supported one. For a tight loop on the backend alone, run the
database in Docker and the API on the host:

```sh
make db-only
cd backend
DATABASE_URL='postgres://energy:energy@127.0.0.1:55433/energy?sslmode=disable' \
  HOST=127.0.0.1 go run ./cmd/server  # match Compose's default published DB port
```

If you override `DB_PORT` or `DB_PASSWORD`, adjust `DATABASE_URL` accordingly.

For the dashboard, `pnpm dev` in `frontend/` proxies the API, so the same routes work
against a server on the host. `make build` rebuilds the image after a change.

## Decisions

The reasoning behind the parts that are not obvious lives in `docs/adr/`, one file per
decision: the detector's authority over the narrator, the episode model, the hourly
baseline, PostgreSQL without TimescaleDB, what confidence means, why analysis persists
before it narrates, the evidence given to the narrator, the run states, derived health,
and why confidence is not averaged into a dashboard headline. `CONTEXT.md` holds the
vocabulary the code and these docs share.

## Limits

Known and deliberate for this MVP: hourly granularity only, so a spike inside one hour is
invisible; one baseline per meter, so a meter that permanently changes its profile is
flagged until it is dismissed; no authentication, so the status endpoints assume a trusted
network; and the OpenAI narrator is a rewrite, so it improves wording rather than adding
analysis the rules did not perform.
