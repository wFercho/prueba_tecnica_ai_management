# Filtros por fecha y paginación en tablas

Status: ready-for-agent

## Problem Statement

La persona que opera el panel no puede acotar los hallazgos ni las tablas horarias a un rango de fechas, y las tablas largas (la serie horaria de un medidor) se presentan enteras sin paginar, lo que dificulta revisar un periodo concreto y moverse por cientos de filas.

## Solution

Añadir filtros de rango Desde/Hasta a las tablas que tienen fecha por fila y paginación a todas las tablas, con todo el filtrado en cliente, controles en español y la página reiniciada ante cualquier cambio de filtro u orden.

## User Stories

1. Como operador, quiero filtrar hallazgos por rango de fechas, para centrarme en los episodios de un periodo concreto.
2. Como operador, quiero que un episodio largo que solapa mi rango siga visible, para no perder hallazgos vigentes por empezar antes del desde.
3. Como operador, quiero acotar la tabla horaria de variables eléctricas a un rango, para revisar un día concreto sin recorrer 336 filas.
4. Como operador, quiero acotar los hallazgos de calidad por la hora del hallazgo, para correlacionarlos con la serie.
5. Como operador, quiero un botón para limpiar el rango y volver al periodo completo, para no quedarme filtrado sin darme cuenta.
6. Como operador, quiero paginar los hallazgos de diez en diez, para avanzar por la lista con controles claros.
7. Como operador, quiero paginar los medidores de diez en diez, para una tabla que crecerá con la flota.
8. Como operador, quiero paginar la serie horaria de veinticuatro en veinticuatro, para avanzar día a día.
9. Como operador, quiero leer «Anterior/Siguiente» y «Página X de Y» en español, para entender la paginación sin traducir.
10. Como operador, quiero que la página vuelva a la primera al cambiar filtros u orden, para no quedarme en una página vacía.
11. Como operador que usa teclado, quiero operar filtros y paginación solo con teclado y foco visible, para no depender del ratón.
12. Como evaluador, quiero que el contrato de la API siga intacto, para que la demo no cambie bajo los pies.

## Implementation Decisions

- El filtrado vive íntegramente en el cliente; la API no gana parámetros de fecha y el contrato no cambia.
- En hallazgos el criterio es solape de la ventana del episodio con el rango; en las tablas horarias, pertenencia de la hora al rango. La tabla de medidores no lleva filtro de fechas por no tener fecha por fila.
- Los filtros son dos inputs de fecha con acción de limpiar, por defecto sin filtrar (periodo completo); sin presets por no haber necesidad demostrada.
- Las tablas con motor de tabla usan su paginación nativa; las tablas planas de solo lectura se cortan con estado local, sin migrarlas de tecnología ni regalarles orden.
- Los controles de paginación van debajo a la derecha con texto en español; cualquier cambio de filtro u orden reinicia a la primera página.
- Sin cambios de backend, esquema, API, detector ni lógica de análisis.

## Testing Decisions

- Una buena prueba verifica comportamiento observable (filas visibles ante un rango, solape de episodios largos, tamaños de página, reset ante filtros, español y teclado), no detalles de implementación (componente de paginación concreto, valores internos de página).
- Costuras, de mayor a menor nivel: recorrido manual de navegador en español como costura principal; compilación y lint del frontend como red de tipos; smoke de solo lectura para confirmar que el contrato no cambió.
- Prior art: el smoke autenticado del contrato API y el recorrido de demo documentado; no se introduce un framework E2E nuevo.

## Out of Scope

- Cambios de backend, esquema, API, detector, confianza, narración o interfaz más allá de filtros y paginación.
- Parámetros de fecha en la API, presets de rango, orden nuevo en tablas planas, paginación en servidor o nuevas vistas.

## Further Notes

- Decisiones tomadas en entrevista previa: solape (no inicio) para no esconder episodios vigentes; veinticuatro filas por página en la horaria por su ritmo diario; corte manual en planas para un blast radius mínimo.
