# Gestión energética · demo local

Aplicación para investigar episodios anómalos en lecturas eléctricas horarias. Un detector
determinista decide el tipo, la severidad y la puntuación de evidencia; un modelo de
lenguaje **opcional** puede mejorar después la redacción en español, sin cambiar el
veredicto ni bloquear la demostración sin API key.

## Demostración en 5–10 minutos

Necesitas Docker y `make`. No hace falta instalar Go, Node ni PostgreSQL en el host.

```sh
make up       # compila y arranca PostgreSQL, API e interfaz en http://localhost:8090
make seed     # importa los CSV sintéticos; reinicia runs y hallazgos previos
```

1. Abre <http://localhost:8090> e inicia sesión con **`admin@email.com` / `admin`**.
   Son credenciales **conocidas de demo local**, nunca aptas para exponer en Internet.
2. En el panel, antes del primer análisis, observa «Pendiente de análisis» y salud
   «Sin analizar». Entra a M-109 por su enlace: hay 336 lecturas y línea base, pero
   **ninguna banda ni clasificación precalculada**. Recarga la ruta directa si quieres.
3. Regresa al panel y pulsa «Ejecutar análisis IA» (o `make analyze`, que inicia sesión con la cuenta demo). La API devuelve `202` solo después
   de guardar las cuatro explicaciones y acciones por reglas en español.
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

Con una pila **aislada** o después del recorrido manual, ejecuta
`SMOKE_ANALYZE=1 make smoke` para verificar los cuatro episodios y el contrato
completo. No lo uses antes de enseñar el botón en la BD de demo. Las pruebas SQL
usan una base descartable separada y nunca borran el volumen de la demo.

Las decisiones de dominio están en `CONTEXT.md` y `docs/adr/`; los tickets y la spec
en `.scratch/energy-management-mvp/`.
