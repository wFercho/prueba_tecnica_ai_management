# 12: Investigación con evidencia y solo acción recomendada

**What to build:** Al abrir cualquier anomalía desde la tabla, el operador ve qué cambió, cuándo, con qué evidencia y qué se recomienda hacer. No se le ofrecen acciones ficticias para reconocer, resolver o descartar hallazgos.

**Blocked by:** 06 — Gráfico temporal legible de M-109; 07 — Investigar la calidad intermitente de M-112; 09 — Conservar el último run exitoso cuando otro falla.

**Status:** resolved

- [ ] El detalle muestra tipo, severidad, banda y score de confianza con los cuatro términos, comparación real/baseline, serie del episodio, variables eléctricas, reporte de contexto o ausencia de evento, hallazgos de calidad y acción recomendada en español.
- [ ] M-106 explica la parada programada y dice «No escalar»; M-109 muestra que `UNKNOWN` no es explicación y queda primero para investigar; M-112 conserva valores ofensores y `status=OK` como afirmación de origen.
- [ ] El frontend representa la forma real de los hallazgos sin esperar campos inexistentes; porcentajes, unidades, horas y etiquetas españolas son coherentes y no se multiplica dos veces la desviación.
- [ ] Se eliminan controles y endpoint de transición humana, incluidos `ACKNOWLEDGED`, `RESOLVED` y `DISMISSED`; las anomalías del MVP siguen `OPEN` y «Acción recomendada» no ejecuta un workflow.
- [ ] Enlaces directos a una anomalía y de vuelta a su medidor funcionan al recargar; un resultado de un run histórico no se presenta como vigente tras otro completado.
- [ ] Tests HTTP/UI del detalle, ausencia del antiguo `PATCH`, reportes y variantes de los cuatro tipos prueban evidencia y recomendación sin inspeccionar implementación privada.
