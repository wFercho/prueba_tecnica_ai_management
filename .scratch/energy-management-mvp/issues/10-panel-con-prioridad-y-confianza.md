# 10: Panel con prioridad, salud y confianza correctas

**What to build:** En pocos segundos el operador ve qué requiere atención: consumo, número de medidores, anomalías, dos de prioridad alta, confianza atribuida al caso más urgente y último intento; M-112 aparece `ALERT` sin perder su anomalía `HIGH`.

**Blocked by:** 09 — Conservar el último run exitoso cuando otro falla.

**Status:** resolved

- [ ] El panel muestra los seis grupos de KPI pedidos: consumo total del periodo, medidores, episodios del último run exitoso, alta prioridad, confianza IA y fecha/hora/estado del último intento.
- [ ] «Alta prioridad» cuenta solo severidad `HIGH` y da 2 en los CSV entregados; `MEDIUM` no se suma a esa cifra.
- [ ] Salud se deriva del mayor impacto operativo de episodios actuales: M-109 `CRITICAL`, M-104/M-112 `ALERT`, M-106 `HEALTHY`; la severidad de M-112 permanece `HIGH` como dato separado.
- [ ] Confianza muestra banda atribuida al medidor más urgente y distribución **calculada** de bandas, no promedio numérico ni distribución fija; score y severidad no se confunden.
- [ ] Antes del análisis o con historia insuficiente no se afirma que el medidor sea sano; fecha y estado del intento fallido no ocultan que los KPIs de hallazgos provienen de resultados anteriores.
- [ ] Tests del seam HTTP sobre resumen y comprobación de UI validan los valores y etiquetas españolas, sin almacenar KPIs derivados.
