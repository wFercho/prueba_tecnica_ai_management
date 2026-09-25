# Historias de Usuario — Prueba Técnica AI Energy Management Platform

Estas 18 historias describen el **comportamiento objetivo del MVP**, no el estado actual del código. La fuente de requisitos es el PDF original; sus tablas y ejemplos ilustran resultados, pero no imponen librerías, frases literales ni cifras calculadas distintas de las lecturas entregadas. El stack frontend definido aquí es una **decisión del proyecto**, no del PDF. Toda la interfaz y el texto generado que recibe el operador deben mostrarse en español. Los ADRs registran las decisiones para implementarlo. La rúbrica global (§17) y la específica de IA (§18) suman 100 puntos **cada una**; no son 100 secciones del documento.

---

## Épica 0 — Fundación

### US-00 · Como evaluador, quiero clonar el repo y arrancar una demo reproducible sin instalar herramientas de desarrollo

**Criterios de aceptación:**
- `make up` construye y levanta PostgreSQL 17 (sin TimescaleDB), API Go y frontend en un solo puerto local, solo con Docker y Make; provisiona la cuenta de demo si aún no existe.
- `make seed` importa `readings.csv` y `events.csv`, restablece las anomalías y los runs derivados, y conserva usuarios y sesiones; repetirlo produce el mismo estado inicial de datos.
- El camino de evaluación es `make up` → `make seed` → abrir la app → iniciar sesión → pulsar «Ejecutar análisis IA». No se ejecuta `make analyze` antes de abrir la app.
- `expected_results.csv` no está en los datos, fixtures ni entrada del detector/LLM, ni se distribuye con la aplicación; está reservado al evaluador.
- El README documenta estos pasos, la cuenta local de demo y un recorrido de 5–10 minutos, sin prometer un tiempo fijo de construcción de imágenes.

**Trazabilidad:** §19–§21, ADR-0004, ADR-0013, ADR-0014.

---

## Épica 1 — Dashboard

### US-01 · Como operador, quiero ver un dashboard con KPIs del periodo para entender el estado general en 5 segundos

**Criterios de aceptación:**
- Muestra al menos: consumo total del periodo, número de medidores, anomalías detectadas, anomalías de alta prioridad, confianza IA (banda + distribución), último análisis (fecha/hora y estado).
- El KPI «Confianza IA» no presenta una media numérica: muestra la banda de la anomalía más urgente atribuida a un medidor (por ejemplo, `Alta (M-109)`) y la distribución **real** de bandas del último run exitoso; no se fija una distribución de ejemplo.
- Antes del primer análisis, «Último análisis: nunca» y «Pendiente de análisis» en contadores de anomalías/prioridad; no `0` que implique datos ya evaluados.
- «Alta prioridad» cuenta exclusivamente anomalías `HIGH` del último run exitoso (2 en el dataset entregado), aunque un medidor con un episodio de calidad `HIGH` muestre salud `ALERT`.
- «Último análisis» muestra fecha/hora y estado del intento más reciente; si falla, los resultados del último run exitoso conservan una advertencia de antigüedad.
- Los KPIs se recalculan en lectura; no se almacenan.

**Trazabilidad:** §5, ADR-0005, ADR-0008, ADR-0010.

### US-02 · Como operador, quiero ver consumo, variación, salud y severidad de anomalía por medidor para identificar cuáles requieren atención

**Criterios de aceptación:**
- Columnas: Medidor, Consumo, Variación, Salud y Anomalía (severidad). El PDF original dice «Anomalía», no «Nivel».
- La salud `{HEALTHY | ALERT | CRITICAL}` se deriva del **mayor impacto operativo** de las anomalías del último run exitoso: M-109 `CRITICAL`, M-104 y M-112 `ALERT`, M-106 `HEALTHY`. La severidad `HIGH` de M-112 aparece por separado.
- Antes del primer análisis o si faltan datos para un baseline fiable, se muestra «Sin analizar»/«Sin datos», nunca `HEALTHY` por ausencia de resultados.
- Filtros: todos, normales, alertas, críticas.
- Búsqueda por `meter_id`.
- Ordenamiento por consumo, variación o severidad.
- Muestra nombre y ubicación para orientar al operador; como no proceden de los CSV entregados, se etiquetan explícitamente como datos sintéticos, no medidos.

**Trazabilidad:** §6, ADR-0009, ADR-0014.

### US-03 · Como operador, quiero ver el detalle de un medidor con su histórico y baseline para entender su comportamiento

**Criterios de aceptación:**
- Encabeza con consumo y baseline **del mismo periodo**, variación porcentual y salud; muestra aparte la última lectura horaria, con su hora y unidad, y el histórico.
- Muestra voltaje, corriente y factor de potencia.
- El gráfico de serie temporal renderiza consumo real y baseline; la banda de la ventana anómala solo aparece después del análisis.
- El tooltip muestra en español hora, consumo real, «Línea base» y desviación; las series tienen leyenda y estilos distinguibles sin depender solo del color, con resumen o tabla accesible de evidencia.
- Para M-109, la ventana de 58 horas se lee como un bloque continuo.
- Antes del primer run se puede inspeccionar M-109 sin etiqueta, ventana ni explicación precalculadas. Se prueba primero Recharts contra los cuatro criterios del gráfico (banda distinguible, tooltip completo, eje de 14 días legible, 58 horas como bloque); si falla un spike acotado, se utiliza SVG manual solo para este gráfico y se documenta el motivo.

**Trazabilidad:** §7, §21, ADR-0012, ADR-0015.

---

## Épica 2 — Motor de Anomalías

### US-04 · Como operador, quiero que el sistema detecte anomalías reales, explicables, falsos positivos y problemas de calidad para no tratar todo igual

**Criterios de aceptación:**
- El detector es determinista y siempre se ejecuta.
- Usa baseline por medidor, desviaciones, consistencia entre variables eléctricas y contexto de eventos para identificar cambios bruscos, cambios persistentes, outliers y defectos de calidad; §8 evalúa el resultado y permite reglas/estadística/ML, no prescribe cinco algoritmos ni un modelo local.
- Detecta correctamente los cuatro casos de §9: M-109 REAL/HIGH, M-112 DATA_QUALITY/HIGH, M-104 EXPLAINABLE/MEDIUM, M-106 FALSE_POSITIVE/LOW.
- Ningún medidor normal produce una anomalía (test de control negativo).
- No contiene reglas especiales por ID del medidor: funciona con otros CSV compatibles. Si falta historia suficiente para un baseline fiable, reporta cobertura parcial y no inventa `HEALTHY` ni una anomalía de consumo.
- La fila `UNKNOWN` de M-109 en `events.csv` es un reporte explícito de **ausencia de evento explicativo**, no un evento que disculpe el cambio. Un reporte `DATA_QUALITY` tampoco sustituye a la evidencia eléctrica necesaria para clasificar M-112.

**Trazabilidad:** §8, §9, §18, ADR-0001, ADR-0003, ADR-0011.

### US-05 · Como evaluador, quiero que el sistema no trate M-106 como anomalía real para verificar que el detector razona sobre eventos operativos

**Criterios de aceptación:**
- M-106 se persiste como fila de primera clase con `type = FALSE_POSITIVE` y `severity = LOW`.
- La UI lo muestra con "No escalar".
- El test automático falla si M-106 se clasifica como `REAL_ANOMALY`.
- La explicación menciona la parada programada de `events.csv`.

**Trazabilidad:** §9, §11, §18 (15 puntos), ADR-0002.

### US-06 · Como evaluador, quiero que el sistema detecte M-112 como problema de calidad de datos y no como anomalía de consumo para verificar que distingue defectos eléctricos de cambios de consumo

**Criterios de aceptación:**
- M-112 se clasifica como `DATA_QUALITY` con `severity = HIGH`.
- La detección usa robust bounds + energy-balance test como disparador.
- El CSV tiene lecturas **cada hora**; las defectuosas reaparecen cada tres horas. Ese patrón y la cuantización del factor de potencia se muestran como hallazgos de calidad separados de los cuatro términos numéricos de `confidence_basis`, no como únicos disparadores.
- La explicación compara valores eléctricos ofensores con el consumo y deja claro que el `status=OK` de origen no certifica consistencia física; el valor original se conserva sin modificar.
- Los hallazgos exponen horas y valores afectados. No se exige un `data_quality_flag` para las 4.032 lecturas.

**Trazabilidad:** §8, §9, §12, §18 (10 puntos), ADR-0002, ADR-0007.

### US-07 · Como operador, quiero que una anomalía sea un episodio, no una fila por lectura, para que la lista sea manejable

**Criterios de aceptación:**
- Una anomalía = una fila por medidor y episodio relacionado, no necesariamente por bloque de lecturas malas contiguas.
- Campos: `window_start`, `window_end`, `affected_reading_count`, `detected_by`; el contador solo incluye lecturas afectadas, no las normales dentro de la ventana.
- Fallos del mismo tipo separados a lo sumo tres horas pueden formar un episodio (umbral configurable); una separación mayor inicia otro.
- M-109 con 58 horas anómalas = 1 fila; M-112 con 16 filas malas = 1 fila; M-106 con 12 = 1 fila.
- El dataset entregado produce exactamente 4 anomalías; otros CSV pueden producir otro número.

**Trazabilidad:** §9, §11, §13, ADR-0002.

---

## Épica 3 — Inteligencia Artificial

### US-08 · Como evaluador, quiero que la IA detecte, clasifique, priorice, explique y recomiende para verificar que el sistema convierte datos en decisión operativa

**Criterios de aceptación:**
- El payload de salida incluye los campos de §10: `meter_id`, `anomaly`, `type`, `severity`, `confidence`, `reason`, `recommended_action`. `anomaly=true` significa que se examinó un **episodio candidato**, incluso cuando `type=FALSE_POSITIVE`; el tipo expresa el veredicto.
- La confianza es un score numérico en [0,1] calculado por el detector a partir de cuatro términos: `deviation`, `event_match`, `corroboration`, `persistence`. Mide evidencia para la **clase asignada**, no probabilidad de anomalía real ni prioridad.
- La API devuelve `confidence_basis` con los cuatro términos.
- La UI muestra la banda categórica (`Alta`, `Media`, `Baja`) en el dashboard y la tabla §11, y el número con su derivación en el detalle.
- La documentación define explícitamente que `confidence` es un score de evidencia no calibrado, no una probabilidad; severidad y confianza se deciden por separado. Los enums técnicos de la API se muestran con etiquetas españolas.

**Trazabilidad:** §10, §11, §18 (explicar/recomendar), ADR-0001, ADR-0005.

### US-09 · Como operador, quiero que la explicación de la IA esté sustentada por evidencia para poder confiar en ella

**Criterios de aceptación:**
- El detalle de anomalía muestra **en español**: qué encontró la IA, variables que cambiaron, comparación contra baseline, eventos relacionados, severidad, confianza, acción recomendada y evidencia que soporta la explicación.
- El contrato de narración incluye identidad del medidor, ventana del episodio, baseline vs real, serie dentro del episodio con consumo, voltaje, corriente, factor de potencia y desviación por hora, contexto de eventos/reporte de ausencia, los cuatro términos de `confidence_basis` y hallazgos de calidad con horas y valores ofensores.
- Con esos datos, una explicación puede señalar horas y variables concretas **solo cuando los valores efectivamente lo demuestren**; no inventa cifras como «se duplicó» si no se verifican.
- Nunca se envía la serie completa (336 horas); solo el episodio acotado.

**Trazabilidad:** §10, §12, ADR-0007.

### US-10 · Como evaluador, quiero que la demo funcione sin API key para verificar que el sistema no depende de un proveedor externo

**Criterios de aceptación:**
- Existe una interfaz `Narrate(ctx, evidence) (Narrative, error)` con dos implementaciones: OpenAI y reglas.
- Sin API key, el motor compone `reason` y `recommended_action` **en español** desde plantillas y la UI muestra la fuente «Reglas» y el estado «Lista» (`rules/READY` en API).
- Con API key, las mismas cuatro clasificaciones se mantienen; la narración en español puede actualizar el texto a `llm/READY`. Si falla o incumple el contrato de idioma/evidencia, permanecen las plantillas españolas con `rules/FAILED`.
- El guion principal se ejecuta íntegro sin key; una segunda pasada con LLM es opcional si hay credenciales.
- Un test con stub de `Narrate` prueba que el fallback sin key produce una tabla §11 completa.

**Trazabilidad:** §8, §10, §21, ADR-0001, ADR-0006.

---

## Épica 4 — Ejecución del Análisis

### US-11 · Como operador, quiero pulsar «Ejecutar análisis IA» y ver el progreso para saber que el sistema está trabajando

**Criterios de aceptación:**
- `POST /ai/analyze` devuelve `202 Accepted` con identificador de run (`run.id`) **después** de detectar y persistir las anomalías con explicación por reglas. El trabajo pendiente, si hay key, es la mejora de la narración.
- `GET /ai/analysis/:id` permite hacer polling del estado.
- El run tiene tres estados: `RUNNING → COMPLETED | FAILED`.
- `COMPLETED` significa detección completada, no narración completada; la doc del endpoint lo explicita.
- Cada anomalía tiene `explanation_status: PENDING | READY | FAILED`: sin key, `rules/READY`; con LLM pendiente, `rules/PENDING`; ante fallo, `rules/FAILED` con texto conservado.
- La UI distingue detección completada de explicaciones pendientes (por ejemplo, «Análisis completado · 4 anomalías · explicando 2/4»).
- Los runs se persisten en `analysis_runs` con `started_at`, `finished_at`, `state`, `anomaly_count`, ventana analizada.
- Si falla un nuevo intento, se muestra `FAILED`; los resultados del último run exitoso siguen visibles con aviso «resultados anteriores», nunca atribuidos al intento fallido.

**Trazabilidad:** §5, §13, §15, ADR-0006, ADR-0008.

### US-12 · Como operador, quiero ver la tabla de anomalías completa inmediatamente después de pulsar el botón para no esperar a la narración

**Criterios de aceptación:**
- La secuencia con el dataset entregado es: detectar → clasificar → persistir 4 anomalías con `explanation_source: "rules"` → 202 → tabla §11 completa → si hay key, narración LLM en segundo plano actualiza `reason` y `recommended_action` por fila.
- Nunca hay una celda vacía en la tabla.
- Un fallo del LLM degrada la redacción, no el resultado.
- TanStack Query gestiona consultas, mutaciones, invalidación y polling; refresca estados y filas a medida que cambian y detiene el polling cuando no quedan narraciones pendientes.
- Al repetir el análisis sin reseed se conserva el historial; tabla, salud y KPIs utilizan el **último run completado**, no duplican filas de ejecuciones anteriores.

**Trazabilidad:** §11, §13, ADR-0006, ADR-0008, ADR-0014.

### US-13 · Como operador, quiero ver la pantalla de Anomalías IA con tipo, severidad, confianza y acción para priorizar

**Criterios de aceptación:**
- Tabla con: Medidor, Tipo, Severidad, Confianza, Acción; encabezados y etiquetas visibles en español, con filtros/orden mediante TanStack Table.
- M-109: Anomalía real / Severidad alta / Confianza alta / Investigar.
- M-112: Calidad de datos / Severidad alta / Confianza alta / Validar.
- M-104: Anomalía explicable / Severidad media / Confianza alta / Validar operación.
- M-106: Falso positivo / Severidad baja / Confianza media **o** alta / No escalar; el «Media/Alta» del PDF es un margen, no una cuarta banda obligatoria.
- El orden por defecto prioriza por severidad y evidencia, con M-109 primero; `confidence` no se interpreta como probabilidad de daño.

**Trazabilidad:** §11, §18.

---

## Épica 5 — Experiencia de Producto

### US-14 · Como evaluador, quiero que la app se sienta como un producto SaaS y no como una colección de pantallas para entender qué aporta la IA en pocos minutos

**Criterios de aceptación:**
- Flujo de demo, con controles visibles en español: Iniciar sesión → Panel general → M-109 → Ejecutar análisis IA → Anomalía → Explicación → Acción recomendada.
- El backend valida cuentas de `users` en PostgreSQL con hash de contraseña; `make up` crea `admin@email.com` / `admin` solo si no existe. Hay soporte para varias cuentas, pero su provisión adicional es administrativa, sin CRUD de usuarios en la UI.
- `sessions` persiste sesiones revocables; la cookie es opaca y HttpOnly. La API exige sesión para datos/análisis, la navegación protege pantallas privadas y «Salir» invalida la sesión en el servidor.
- Por defecto Docker publica **API y PostgreSQL** solo en `127.0.0.1`; se advierte que `admin/admin` y las credenciales por defecto de la BD son exclusivos de una demo local y no aptos para despliegue público.
- Stack frontend acordado: React + TypeScript + Vite; Tailwind CSS 4 para estilos; TanStack Router para rutas protegidas y enlaces directos a medidores/anomalías; TanStack Query para estado remoto y polling; TanStack Table para filtros/orden; Recharts para gráficos con el fallback acotado de US-03. TanStack Start, Form y Virtual quedan fuera por no aportar al MVP.
- Todo el texto de producto visible está en español: navegación, login, errores, validaciones, estados, tablas, leyendas, tooltips y recomendaciones; fechas/horas/unidades se presentan con formato español y zona horaria identificable. Los IDs, `status=OK` y enums originales se conservan como datos fuente, con traducción de sus etiquetas. Los reportes entregados en inglés se explican en español sin modificar los CSV.
- Acceso con teclado y etiquetas comprensibles sin depender solo del color; una librería no sustituye las comprobaciones de accesibilidad y UX. El PDF no impone estas herramientas: son elecciones del proyecto registradas en ADR-0015.

**Trazabilidad:** §3, §14, §17 (Frontend/UX), §21, ADR-0012, ADR-0013, ADR-0015.

### US-15 · Como evaluador, quiero que el análisis corra delante de mí para verificar que no está pre-cocinado

**Criterios de aceptación:**
- Tras `make seed`, `anomalies` y `analysis_runs` están vacías; `users` y `sessions` no se borran.
- El dashboard muestra «Último análisis: nunca» y contadores «Pendiente de análisis» hasta que se pulsa el botón.
- La ficha M-109 anterior al análisis muestra lecturas y baseline, sin banda de anomalía ni diagnóstico; tras el run aparece la ventana del episodio.
- El README contiene un recorrido de 5–10 minutos con clics y propósito de cada paso; el LLM es una comparación opcional, no un prerrequisito.

**Trazabilidad:** §13, §21, ADR-0006, ADR-0014.

---

## Épica 6 — Calidad

### US-16 · Como evaluador, quiero que los cuatro casos de §9 estén cubiertos por tests automáticos para verificar que el detector es correcto

**Criterios de aceptación:**
- Tests table-driven en Go sobre el paquete de detección.
- Los cuatro casos de §9 sobre los CSV entregados; M-104 (96 lecturas), M-106 (12), M-109 (58) y M-112 (16) forman cuatro episodios.
- Los 8 medidores normales como conjunto de control negativo.
- Un test falla si M-109 se omite, si M-106 se clasifica `REAL_ANOMALY`, o si cualquier medidor normal produce una anomalía.
- Tests contrafactuales con IDs/fechas diferentes, sin el reporte M-112 y con huecos >3 horas: el resultado depende de la evidencia, no de identificadores o etiquetas de `events.csv`.
- Tests HTTP para login, protección de endpoints, logout revocable, estados/códigos, JSON y selección del último run exitoso cuando otro falla; el texto por reglas y el fallback son españoles.
- Revisión visual del recorrido completo: rutas directas y retorno tras login, filtros/orden, polling por fila, gráfico y tooltip accesibles, logout y **todo el texto visible en español**.
- Test con stub de `Narrate` que prueba la tabla completa sin key, actualización por fila con LLM y preservación de plantillas si falla.

**Trazabilidad:** §8, §9, §17, §18, ADR-0001, ADR-0002, ADR-0006, ADR-0013.

### US-17 · Como evaluador, quiero documentación que explique las decisiones para entender por qué el sistema es como es

**Criterios de aceptación:**
- README actualizado **cuando se implemente**: dos comandos para la demo, credenciales locales y su riesgo, guion 5–10 minutos, datos sintéticos identificados, comportamiento sin key y ruta opcional con LLM; documenta el stack elegido y la política de idioma español.
- Documentación de persistencia acorde con el esquema real: PostgreSQL sin TimescaleDB; `meters`, `readings`, `events`, `analysis_runs`, `anomalies`, `users`, `sessions`. Los datos de la prueba son 4.032 lecturas, 336 por medidor; no se exige crear un `consideraciones_persistencia.md` que hoy no existe.
- ADRs concordantes con el PDF y el producto: 0001 (detector/LLM), 0002 (episodio), 0004 (PostgreSQL), 0005 (confianza), 0006 (narración), 0008 (run), 0009 (salud/severidad), 0010 (KPI confianza), 0012 (gráfico), 0013 (login demo), 0014 (reset del seed) y 0015 (stack frontend/idioma). 0011 trata umbrales; el stack es una decisión del proyecto, no una exigencia del PDF.
- `CONTEXT.md` distingue `status` de origen preservado, hallazgos de calidad, estado de anomalía `OPEN` (sin workflow humano en el MVP), salud del medidor, severidad, prioridad, confianza no probabilística, episodio y reporte de ausencia de evento.

**Trazabilidad:** §14, §16, §17 (Testing/documentación/calidad), §20, ADR-0004, ADR-0013, ADR-0014, ADR-0015.

---

## Resumen de trazabilidad

| Épica | Historias | Secciones del spec | Puntos §17/§18 |
| :--- | :--- | :--- | :--- |
| Fundación | US-00 | §19–§21 | — |
| Dashboard | US-01, US-02, US-03 | §5, §6, §7 | 20 (Frontend/UX) |
| Motor de anomalías | US-04, US-05, US-06, US-07 | §8, §9, §11 | 15 (Detección) + 15 (M-106) + 10 (M-112) |
| IA | US-08, US-09, US-10 | §10, §12, §18 | 30 (M-109) + 25 (priorizar) + 20 (explicar/recomendar) |
| Ejecución | US-11, US-12, US-13 | §11, §13 | — |
| Producto | US-14, US-15 | §3, §14, §21 | — |
| Calidad | US-16, US-17 | §17 | 10 (Testing/docs) |

**Total: 18 historias de usuario (US-00 a US-17)**. La tabla agrupa trazabilidad sin sumar dos veces las rúbricas independientes de §17 y §18. Son criterios de aceptación para implementar y comprobar; no certifican que la aplicación actual ya los cumpla.
