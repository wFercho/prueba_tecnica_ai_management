# 05: Ejecutar análisis y ver cuatro episodios inmediatamente

**What to build:** Tras pulsar «Ejecutar análisis IA», el evaluador ve una tabla completa y priorizada de los cuatro episodios del dataset aun sin API key: M-109 primero, M-112 como calidad, M-104 explicable y M-106 como falso positivo que no se escala.

**Blocked by:** 04 — Reimportar datos sin resultados precalculados.

**Status:** ready-for-agent

- [ ] La petición de análisis responde `202` con el identificador de run después de persistir las cuatro anomalías y sus explicaciones/acciones por reglas **en español**; la tabla se puede consultar inmediatamente y ninguna celda de razón o acción está vacía.
- [ ] Los tipos/severidades son los cuatro casos del PDF; hay exactamente cuatro episodios para los CSV entregados y ninguno en los ocho medidores de control. M-109 encabeza el orden, M-106 queda `FALSE_POSITIVE/LOW` con «No escalar».
- [ ] La API expone `anomaly=true` como «episodio candidato examinado» incluso para M-106, además de tipo, severidad, score, banda y cuatro términos de confianza; los enums se etiquetan en español en la UI.
- [ ] Sin key cada fila queda `rules/READY`. La tabla usa TanStack Query para recibir los resultados y TanStack Table para la presentación/orden inicial sin inventar una banda «Media-Alta».
- [ ] Tests del detector con los CSV y del seam HTTP con almacenamiento en memoria comprueban los cuatro casos, el `202` posterior a persistencia, el JSON y la tabla completa sin proveedor externo.
