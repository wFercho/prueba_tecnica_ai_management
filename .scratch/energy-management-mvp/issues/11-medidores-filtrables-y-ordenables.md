# 11: Tabla de medidores filtrable y ordenable

**What to build:** El operador localiza un medidor por ID y compara consumo, variación, salud y severidad de anomalía en una tabla con filtros de salud y ordenación, sin confundir datos sintéticos con lecturas reales.

**Blocked by:** 10 — Panel con prioridad, salud y confianza correctas.

**Status:** resolved

- [ ] La tabla muestra identificador, nombre/ubicación etiquetados como sintéticos, consumo del periodo, variación contra baseline comparable, salud y columna «Anomalía» con severidad separada.
- [ ] TanStack Table ofrece filtros Todos/Normales/Alertas/Críticos, búsqueda por `meter_id` y orden por consumo, variación y severidad; el estado sin análisis y la salud insuficientemente evaluada no aparecen como «Normal».
- [ ] M-112 muestra `ALERT/HIGH`; M-106 es `HEALTHY/LOW` tras analizar, y antes del primer run todos muestran estado no evaluado, no una falsa salud.
- [ ] Filas y controles se pueden usar con teclado, permiten navegar al detalle y tienen encabezados/indicadores de orden legibles en español.
- [ ] Tests del contrato de variación/salud y un recorrido UI de filtro, búsqueda, orden y navegación prueban el comportamiento sin acoplarse a detalles internos de TanStack Table.
