# 13: Mejora LLM opcional en español, por fila

**What to build:** Si el evaluador proporciona una API key, ve mejorar una a una las explicaciones en español sin que cambien tipos, severidades, confianza ni acciones útiles durante la espera. Sin key o con el proveedor fallando, el recorrido offline sigue completo.

**Blocked by:** 12 — Investigación con evidencia y solo acción recomendada.

**Status:** ready-for-agent

- [ ] Sin key las filas quedan `rules/READY` con razón y recomendación españolas; con key pasan por `rules/PENDING` hasta `llm/READY` o conservan reglas como `rules/FAILED` si el proveedor falla.
- [ ] El LLM recibe solo identidad, ventana y lecturas del episodio (consumo, baseline, desviación, voltaje, corriente y factor de potencia por hora), reporte de contexto/ausencia, cuatro términos de confianza y hallazgos de calidad; no recibe la serie completa de 336 horas.
- [ ] Una narración inventada, vacía, no sustentada por los datos o no española no reemplaza la plantilla; el LLM no decide ni modifica tipo, severidad o score.
- [ ] TanStack Query actualiza filas, detalle y progreso mientras cada respuesta llega y detiene el polling cuando termina; `COMPLETED` del run sigue significando detección terminada, no narración terminada.
- [ ] Tests con narrador stub que responde, se demora, devuelve inglés o falla prueban texto español persistido antes del `202`, actualización independiente por fila y fallback visible sin celdas vacías.
