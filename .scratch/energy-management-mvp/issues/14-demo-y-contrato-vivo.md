# 14: Recorrido de demo y contrato vivo verificables

**What to build:** Una persona ajena al equipo clona el repo, ejecuta dos comandos, entra, explora M-109, ejecuta el análisis y llega a una acción recomendada en 5–10 minutos sin sorpresas de idioma, datos precocinados ni credenciales externas obligatorias.

**Blocked by:** 03 — Salir revocando la sesión; 08 — Analizar CSV compatibles sin memorizar medidores; 11 — Tabla de medidores filtrable y ordenable; 13 — Mejora LLM opcional en español, por fila.

**Status:** resolved

- [ ] El README en español explica `make up` y `make seed`, la cuenta y contraseñas de demo solo local, los datos sintéticos, la opción LLM y un guion de 5–10 minutos con navegación concreta; no ordena ejecutar un análisis antes de abrir la app.
- [ ] El smoke autenticado comprueba API y formas JSON que lee el frontend: cuatro episodios, dos `HIGH`, M-112 `ALERT/HIGH`, confianza no promediada, `OPEN`, hallazgos de calidad y ausencia de `PATCH` de workflow.
- [ ] El smoke corre en entorno aislado o después del recorrido de demo, de modo que no rellena anomalías en la BD con la que se quiere mostrar el primer análisis.
- [ ] Una revisión manual del navegador cubre login, ruta directa, estado previo al run, botón y progreso, cuatro anomalías, gráfico/tooltip accesibles, filtros y orden, narración por fila, acción recomendada y logout; todo el texto visible se verifica en español.
- [ ] Se ejecutan las pruebas de detector, HTTP, PostgreSQL y build frontend; los ejemplos de README reflejan el comportamiento real. `expected_results.csv` no está disponible para la app, los tests ni el LLM.
