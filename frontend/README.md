# Frontend

The dashboard uses React 19, Vite, TanStack Router for shareable URLs, TanStack
Query for server data and Tailwind CSS 4 for the navigation frame and meter view.
Existing CSS remains for views that have not yet been migrated. The chart is still
an SVG component.

It normally is not run on its own — the API serves its build output, so the dashboard
and the API share an origin and there is no CORS anywhere:

```sh
make up && make seed && make analyze   # from the repository root
open http://localhost:8090
```

From the panel, the meter ID links to `/medidores/M-109`. That URL can also be
opened directly or refreshed; the breadcrumb returns to `/`, and browser Back
returns to the previous location. Finding URLs use `/hallazgos/{id}`. These UI
paths do not overlap the JSON API paths, such as `/meters/M-109/readings`.

## Working on it

With the stack up (`make up`), run the dev server here and it proxies the API's routes
to the container, so `/meters` and friends work exactly as they do in production:

```sh
pnpm install
pnpm dev              # http://localhost:5173, proxies to http://localhost:8090
API_PORT=9090 pnpm dev  # if Compose publishes the API on a different port
API_URL=http://localhost:9090 pnpm dev  # or specify the complete proxy target
```

The browser always uses relative API paths; `API_URL` and `API_PORT` configure
only the Vite development proxy. With the built dashboard, the backend serves
both the frontend and the API from the same origin.

`pnpm build` type-checks and builds into `dist/`, which the Docker build copies into the
image. `pnpm lint` runs the project's ESLint config.

## Shape of the code

| File | What it holds |
| :--- | :--- |
| `src/api.ts` | The types, mirroring the JSON the API actually answers, and one function per route |
| `src/router.tsx` | Typed page routes and deep-link parameters |
| `src/App.tsx` | Shared Spanish navigation frame |
| `src/views/Dashboard.tsx` | The KPI row, the findings list, and running the detector with polling |
| `src/views/MeterView.tsx` | One meter: its series against its baseline, its anomalies, its events |
| `src/views/AnomalyView.tsx` | The investigation view: evidence, score, prose, and the status decision |
| `src/components/ConsumptionChart.tsx` | The SVG chart |
| `src/components/badges.tsx` | The shared badges and the formatters |
| `src/index.css` | Tailwind imports and legacy view styles |
| `smoke.mjs` | A contract walk of the running API: `make smoke` from the root |

Two things are deliberate. Timestamps are formatted in UTC, because the readings are and
an episode the detector calls 14:00 must not be drawn at a different hour. And confidence
is shown as a two-decimal score with its band, never as a percentage of being right,
because it is an evidence score rather than a probability.

`smoke.mjs` exists because the TypeScript types prove nothing at runtime. It walks the
live API and asserts that every field these views read exists, has the right type, and
carries the value the UI claims. It is how the mismatch between the backend's `ALERT`
health value and the frontend's `WARNING` was found.
