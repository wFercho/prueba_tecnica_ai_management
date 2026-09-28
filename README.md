# Gestión energética · demo local

Aplicación para investigar episodios anómalos en lecturas eléctricas horarias. Un detector
determinista decide el tipo, la severidad y la puntuación de evidencia; un modelo de
lenguaje **opcional** puede mejorar después la redacción en español, sin cambiar el
veredicto ni bloquear la demostración sin API key.

## Demostración

Necesitas Docker y `make`. No hace falta instalar Go, Node ni PostgreSQL en el host.

```sh
make demo     # vía rápida: levanta la pila e importa los CSV sintéticos
```

Equivale a ejecutar los dos pasos por separado, que siguen disponibles como
control fino:

```sh
make up       # compila y arranca PostgreSQL, API e interfaz en http://localhost:8090
make seed     # importa los CSV sintéticos; reinicia runs y hallazgos previos
```

Ninguna de las dos vías lanza el análisis: la demo debe mostrar el primer
análisis ocurriendo en la interfaz.

1. Abre <http://localhost:8090> e inicia sesión con **`admin@email.com` / `admin`**.
   Son credenciales **conocidas de demo local**, nunca aptas para exponer en Internet.
2. En el panel, antes del primer análisis, observa «Pendiente de análisis» y salud
   «Sin analizar». Entra a M-109 por su enlace: hay 336 lecturas y línea base, pero
   **ninguna banda ni clasificación precalculada**. Recarga la ruta directa si quieres.
3. Regresa al panel y pulsa «Ejecutar análisis IA». La API devuelve `202` solo después
   de guardar las cuatro explicaciones y acciones por reglas en español. El botón
   es la vía principal de la demo; `make analyze` invoca la misma ruta
   (`POST /ai/analyze`, iniciando sesión con la cuenta demo) para lanzar esa
   misma lógica desde la terminal.
4. Investiga los cuatro episodios: M-109 (`REAL_ANOMALY/HIGH`, 58 horas), M-112
   (`DATA_QUALITY/HIGH`, 16 lecturas intermitentes), M-104 (`EXPLAINABLE/MEDIUM`)
   y M-106 (`FALSE_POSITIVE/LOW`, «No escalar»). Los otros ocho medidores no generan
   hallazgos. Salud y severidad no son lo mismo: **M-112 es `ALERT/HIGH`**.
5. Filtra o busca medidores, abre un hallazgo y revisa la hora, la serie, los términos
   de confianza, el reporte de contexto y la **acción recomendada**. Esta última no
   ejecuta ningún workflow. Prueba «Cerrar sesión»: invalida la cookie anterior.

La cuenta demo se crea solo si no existía y su contraseña se guarda como hash en
PostgreSQL. `make seed` conserva usuarios y sesiones, pero reemplaza medidores, lecturas,
eventos, runs y anomalías **en una transacción**. Repetirlo reinicia el recorrido; no
importa resultados esperados ni llama al detector por adelantado. Todos los nombres y
ubicaciones de medidores son **metadatos sintéticos**, no mediciones del CSV.

## Arquitectura y contrato

- `backend/cmd/server`: API Go y archivos compilados de React desde un mismo origen.
- `backend/cmd/seed`: importación explícita de `data/readings.csv` y `data/events.csv`.
- `backend/internal/analysis`: baseline horario por medidor, episodios, calidad física,
  clasificación y confianza reproducible. `UNKNOWN` es un reporte de ausencia, no una
  explicación del aumento de M-109. `status=OK` es la afirmación original de la lectura,
  no el veredicto derivado de calidad.
- `backend/internal/store`: contrato compartido por implementación en memoria y por
  PostgreSQL. El último **intento** puede estar `FAILED` y coexistir con los hallazgos
  del último run **completado**; nunca se suman runs antiguos como si fueran actuales.
- `frontend`: React, TanStack Router/Query/Table, Tailwind CSS y Recharts. El gráfico
  distingue línea de consumo, línea base discontinua y banda del episodio; también
  ofrece una tabla de lecturas accesible.

### Diagramas

```mermaid
flowchart LR
    browser["Navegador\n(dashboard)"] --> api["API Go\n+ dashboard compilado"]
    api --> db[("PostgreSQL")]
    seed["seed\n(importación)"] --> db
    provision["provision\n(cuentas)"] --> db
```

El navegador habla con un solo origen: la API sirve los datos y el dashboard
compilado. `seed` y `provision` son jobs de un solo uso contra la misma base;
las utilidades de pruebas quedan fuera por no ser runtime.

```mermaid
sequenceDiagram
    actor E as Evaluador
    participant T as Terminal/Compose
    participant N as Navegador
    participant A as API
    participant D as PostgreSQL
    E->>T: make demo (up + seed)
    T->>A: arranque y migración
    A->>D: provisiona demo solo si no existe
    T->>D: job seed importa lecturas y reportes (sin runs)
    E->>N: login demo
    N->>A: POST /auth/login
    A->>D: verifica hash y crea sesión
    E->>N: «Ejecutar análisis IA»
    N->>A: POST /ai/analyze
    A->>D: persiste 4 episodios con prosa por reglas
    A-->>N: 202 con el run
    N->>A: GET /ai/analysis/{id} (polling)
    A->>D: narración opcional por fila
    E->>N: investiga y lee la acción recomendada
    E->>N: cerrar sesión
    N->>A: POST /auth/logout
    A->>D: revoca la sesión
```

La confianza es una puntuación de evidencia en `[0,1]`, no probabilidad de avería.
Se muestran las cuatro bases del score (`deviation`, `event_match`, `corroboration`,
`persistence`), una banda atribuida al episodio más urgente y la distribución real
de bandas. Alta prioridad cuenta únicamente severidad `HIGH` (dos en los CSV).

Todas las rutas de datos exigen cookie de sesión HttpOnly y revocable. Solo el login,
el health check y los recursos estáticos son públicos. La app no incluye CRUD de
usuarios ni acciones para reconocer, resolver o descartar anomalías: solo `OPEN` y
propuestas de acción. Se pueden provisionar más cuentas con un comando administrativo;
no hay registro público:

```bash
export ACCOUNT_EMAIL='otra@ejemplo.com'
read -r -s -p 'Contraseña nueva: ' ACCOUNT_PASSWORD; echo
export ACCOUNT_PASSWORD
docker compose --profile provision run --rm -e ACCOUNT_EMAIL -e ACCOUNT_PASSWORD provision
unset ACCOUNT_PASSWORD
```

Una cuenta existente no se sobrescribe. El secreto no se pasa como argumento ni se
incluye en el archivo Compose; estas instrucciones están pensadas solo para una
terminal de administración local.

| Método | Ruta | Uso |
| --- | --- | --- |
| `POST` | `/auth/login` | Emite cookie de sesión |
| `GET` | `/auth/session` | Consulta usuario autenticado |
| `POST` | `/auth/logout` | Revoca la sesión |
| `GET` | `/health` | Comprobación pública sin datos |
| `GET` | `/meters`, `/meters/{id}`, `/meters/{id}/readings` | Medidores y series |
| `GET` | `/anomalies`, `/anomalies/{id}` | Hallazgos vigentes y evidencia |
| `POST` | `/ai/analyze` | Persiste resultados por reglas y devuelve `202` |
| `GET` | `/ai/analysis/{id}` | Estado de detección y narración de ese run |
| `GET` | `/dashboard/summary` | KPIs, medidores, último intento y último éxito |

## Modelo de datos

Vocabulario de `CONTEXT.md`, entre paréntesis la tabla física. Nombres y
ubicaciones de medidores son metadatos sintéticos.

```mermaid
erDiagram
    meters ||--o{ readings : tiene
    meters ||--o{ events : reporta
    analysis_runs ||--o{ anomalies : contiene
    meters ||--o{ anomalies : afecta
    meters {
        TEXT meter_id
        TEXT name
        TEXT location
    }
    readings {
        TEXT meter_id
        TIMESTAMPTZ timestamp
        DOUBLE consumption_kwh
        DOUBLE voltage_v
        DOUBLE current_a
        DOUBLE power_factor
        TEXT status
    }
    events {
        TEXT meter_id
        TIMESTAMPTZ timestamp
        TEXT type
        TEXT description
    }
    analysis_runs {
        INT id
        TEXT state
        TIMESTAMPTZ window_start
        TIMESTAMPTZ window_end
    }
    anomalies {
        INT run_id
        TEXT meter_id
        TIMESTAMPTZ window_start
        TEXT type
        TEXT severity
        DOUBLE confidence
    }
```

| Concepto (tabla) | Clave / identidad | Nota |
| --- | --- | --- |
| Medidor (`meters`) | `UNIQUE(meter_id)` | Identidad estable (`M-101`…); `name/location` sintéticos |
| Lectura (`readings`) | `UNIQUE(meter_id, timestamp)` | Una hora medida por medidor; `status` es lo que afirmó la fuente (`OK` en las 4.032 filas), nunca el veredicto |
| Reporte de contexto (`events`) | `UNIQUE(meter_id, timestamp, type)` | Reimporte idempotente; `UNKNOWN` es reporte de ausencia, no explicación |
| Run (`analysis_runs`) | `window_start/end` del dataset | `COMPLETED` = detección lista; la narración puede seguir pendiente |
| Anomalía (`anomalies`) | `UNIQUE(run_id, meter_id, window_start)` | Un episodio clasificado; reintentar el análisis crea un run nuevo, no duplica filas |
| Evidencia persistida (`anomalies.confidence_basis/deviation_series/correlated_event/findings`) | JSONB por fila | Serie que juzgó el detector, congelada; no se recalcula en lectura |
| Explicación (`anomalies.reason/recommended_action`) | `explanation_source/status` | Siempre hay prosa por reglas en español; LLM solo puede reescribirla |
| Cuentas (`users/sessions`) | Fuera del dominio | `make seed` las conserva y reemplaza el resto en una transacción |

## Cómo se genera el análisis («análisis IA»)

Dos fases con nombres canónicos: **detección determinista** (decide) y
**narración LLM opcional** (redacta, no decide).

```mermaid
flowchart TD
    A["POST /ai/analyze<br/>StartRun RUNNING"] --> B["detect: AllReadings + Events"]
    B --> C["Analyze por medidor ordenado"]
    C --> D["FindConsumptionEpisodes<br/>baseline: mediana por hora del mismo medidor,<br/>historia limpia, min 3 muestras, congelada"]
    C --> E["FindDataQualityEpisodes<br/>consistencia fisica V/I/PF"]
    D --> F["Correlate: eventos -24h"]
    E --> F
    F --> G["Classify: tipo + severidad + confianza 4 terminos<br/>+ prosa por reglas en español"]
    G --> H["SaveAnomalies + FinishRun COMPLETED<br/>202 con el run"]
    H --> I{"OPENAI_API_KEY?"}
    I -- "no" --> J["rules/READY"]
    I -- "si" --> K["narrateRun en background, por fila<br/>solo reescribe reason/action en español<br/>si falla, conserva reglas FAILED"]
```

Baseline en una línea: esperado de ese medidor a esa hora
(`M-106 00h = 42.110 kWh`, mediana de sus `00h` limpias); `08/09 00h = 8.2`
es `-80.5%`, episodio `00-11h -79.8%`, explicado por `SCHEDULED_OUTAGE`
→ `FALSE_POSITIVE/LOW`.

## Configuración

La pila funciona sin `.env`. Compose lee variables exportadas o un `.env` local
(ignorado por Git). **Los puertos de la API y la BD se publican solo en
`127.0.0.1` por defecto.** `BIND_HOST=0.0.0.0` expondría contraseñas conocidas;
no hagas eso con esta demo.

| Variable | Valor por defecto | Función |
| --- | --- | --- |
| `BIND_HOST` | `127.0.0.1` | Interfaz del host para ambos puertos publicados |
| `API_PORT` / `DB_PORT` | `8090` / `55433` | Puertos publicados por Compose |
| `DB_PASSWORD` | `energy` | Contraseña conocida **solo para demo local** |
| `DEMO_PASSWORD` | `admin` | Contraseña del usuario demo **solo en su primer alta** |
| `OPENAI_API_KEY` | vacío | Sin key, explicaciones por reglas completas |
| `OPENAI_MODEL` | `gpt-4o-mini` | Modelo opcional de narración |
| `OPENAI_BASE_URL` | vacío | Endpoint opcional para probar un stub local |
| `LOG_LEVEL` | `info` | Nivel de logging (`debug`, `info`, `warn`, `error`) |
| `DATABASE_URL` | Compose: `postgres://energy:energy@db:5432/energy?sslmode=disable` | Conexión del backend; adapta el password si cambias `DB_PASSWORD` |
| `HOST` / `PORT` | vacío (todas las interfaces) / `8080` | Listener **interno** del binario Go; distinto de `BIND_HOST` / `API_PORT` |
| `DATA_DIR` / `STATIC_DIR` | `data/` / `frontend/dist/` | CSV y frontend compilado |
| `API_URL` | `http://localhost:${API_PORT:-8090}` | Proxy de Vite **solo en desarrollo**, nunca URL embebida en la app |

El frontend compilado llama rutas relativas desde el mismo origen y no requiere
configurar una URL de API. Para desarrollar el frontend por separado, desde
`frontend/` ejecuta `pnpm install && pnpm dev`; si cambias el puerto publicado
usa `API_PORT=9090 pnpm dev` o `API_URL=http://localhost:9090 pnpm dev`.

Para ejecutar solo el backend en el host con la BD de Compose:

```sh
make db-only
cd backend
DATABASE_URL='postgres://energy:energy@127.0.0.1:55433/energy?sslmode=disable' \
  HOST=127.0.0.1 go run ./cmd/server
```

## Comprobaciones

```sh
make test              # Go con detector de carreras y PostgreSQL de pruebas
make vet
cd frontend && pnpm build && pnpm lint
make smoke             # contrato autenticado, solo lectura: NO lanza el primer análisis
```
