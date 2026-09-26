# 08: Analizar CSV compatibles sin memorizar medidores

**What to build:** Un operador puede analizar lecturas de medidores con otros IDs o fechas y recibir episodios delimitados por evidencia; cuando faltan lecturas para un baseline fiable, ve «Sin datos suficientes» en vez de una clasificación de consumo inventada.

**Blocked by:** 07 — Investigar la calidad intermitente de M-112.

**Status:** resolved

- [ ] Casos contrafactuales con IDs/fechas distintos mantienen el razonamiento de los cuatro tipos sin reglas por identificador y sin acceso a ground truth reservado.
- [ ] M-112 sigue detectándose sin el reporte de calidad; `UNKNOWN` para un caso como M-109 se entiende como reporte de **ausencia** y no como evento que excuse la desviación.
- [ ] Hallazgos del mismo tipo separados hasta tres horas se agrupan con contador de lecturas realmente afectadas; un hueco mayor crea otro episodio, sin fusionar medidores/tipos distintos.
- [ ] Un medidor sin historia fiable recibe cobertura insuficiente en la respuesta y en su vista; no se marca `HEALTHY`, no recibe baseline global ni anomalía de consumo imaginaria, y los demás medidores siguen analizándose.
- [ ] Tests table-driven del detector y un recorrido HTTP/UI con datos alternativos prueban estos resultados observables y mantienen los ocho controles del dataset entregado sin anomalías.
