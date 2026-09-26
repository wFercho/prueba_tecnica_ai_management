# Pulido visual de tablas y evidencia

Status: ready-for-agent

## Problem Statement

La persona que opera el panel duda dónde pulsar en «Hallazgos, por urgencia»: solo el identificador del medidor es un enlace y nada indica que la fila lleve al detalle. Los encabezados ordenables parecen botones sin estilo (texto con subrayado al pasar el ratón y `↑/↓` en texto plano) en lugar de cabeceras de tabla, y los filtros de «Medidores» comparten ese aspecto. El panel y la ficha viven en un contenedor estrecho con una columna lateral fija que comprime tablas de cinco y seis columnas, incluida la de variables eléctricas horarias. La tarjeta de corroboración muestra identificadores crudos en inglés (`energy_balance`, `power_factor`…), lo que rompe la regla de producto de interfaz en español.

## Solution

Hacer clicable lo clicable con una columna de acción visible («Ver» con flecha), dar a los encabezados jerarquía real de cabecera con iconos de orden que reflejen su estado, aprovechar el ancho de pantalla para las tablas y la evidencia horaria, y mostrar las variables que corroboran con etiquetas españolas y un fallback legible. Todo con la librería de iconos `lucide-react`, texto visible en español y sin tocar el backend ni el contrato de la API.

## User Stories

1. Como operador, quiero distinguir a simple vista qué filas de hallazgos llevan al detalle, para no dudar dónde pulsar.
2. Como operador, quiero un control «Ver» con flecha en cada hallazgo, para abrir la investigación con un solo foco de teclado por fila.
3. Como operador que usa teclado, quiero que la navegación por las tablas tenga un orden de foco predecible y foco visible, para no perderme entre controles.
4. Como operador, quiero que los encabezados ordenables se lean como cabeceras (mayúsculas, tenues) y no como botones, para entender la tabla de un vistazo.
5. Como operador, quiero iconos que indiquen si una columna no está ordenada, está ascendente o descendente, para saber qué vista tengo delante.
6. Como operador, quiero buscar y filtrar medidores con controles de la misma altura e iconos coherentes, para no confundirlos con el contenido.
7. Como operador, quiero las tablas del panel a ancho completo, para comparar consumo, variación, salud y severidad sin columnas comprimidas.
8. Como operador, quiero la tabla horaria de variables eléctricas a ancho completo bajo el gráfico y visible sin desplegar nada, para revisar voltaje, corriente y factor de potencia junto a la serie.
9. Como operador, quiero leer las variables que corroboran en español (consumo, corriente, voltaje, factor de potencia, balance energético), para no traducir identificadores mentalmente.
10. Como operador, quiero que un valor futuro desconocido se muestre legible y nunca como `snake_case` crudo, para seguir confiando en la tarjeta.
11. Como operador, quiero leer en español también los textos de accesibilidad (etiquetas de gráficos y controles), para recorrer la app sin alternar idiomas.
12. Como evaluador, quiero que el contrato de la API y los cuatro episodios del dataset sigan intactos tras el pulido, para que la demo no cambie bajo los pies.

## Implementation Decisions

- La tabla de hallazgos gana una columna final de acción («Detalle» con enlace «Ver» y flecha); el identificador del medidor deja de presentarse como enlace para que haya una sola affordance de navegación por fila. El orden operativo que sirve la API se conserva por defecto.
- Iconos con `lucide-react` mediante imports nombrados (aprovechan su tree-shaking): flecha para la acción, iconos de orden ascendente/descendente/sin ordenar, lupa en la búsqueda y cheurón en el selector. Es una decisión de este proyecto para estos controles, no un sistema de diseño completo.
- Los encabezados ordenables adoptan la jerarquía de cabecera existente del proyecto y conservan `aria-sort` y etiquetas accesibles en español; el icono refleja el estado real de orden de cada columna.
- El panel pasa a ancho completo con hallazgos y medidores apilados y el bloque de análisis al costado o debajo según el ancho; la ficha del medidor amplía su contenedor y su columna lateral; la tabla de variables eléctricas sale de la columna estrecha a ancho completo bajo el gráfico, sin pestañas nuevas.
- La tarjeta antes titulada «Corroboración» pasa a «Variables que corroboran», con el mapa español acordado; «Corroboración» queda reservada al término del score de confianza según el glosario. El fallback para valores desconocidos es texto legible en mono.
- Sin cambios de backend, esquema, API ni lógica del detector: el mapa de etiquetas vive en el frontend y la verificación de los cuatro episodios no se altera.

## Testing Decisions

- Una buena prueba verifica comportamiento observable (etiquetas visibles en español, iconos que reflejan el orden, navegación por fila, tablas legibles a ancho completo, foco por teclado), no detalles de implementación (qué componente de icono, valores exactos de CSS).
- Costuras, de mayor a menor nivel: recorrido manual de navegador en español (login, panel con orden y filtros, ficha M-109 con gráfico y variables, hallazgo con evidencia y variables que corroboran, logout, solo teclado) como costura principal; compilación y lint del frontend como red de tipos; smoke de solo lectura para confirmar que ningún campo que lee la UI cambió.
- Prior art: el smoke autenticado del contrato API, las pruebas HTTP del seam servicio y el recorrido de demo documentado; no se introduce un framework E2E nuevo para este pulido.

## Out of Scope

- Cambios de backend, esquema, API, detector, confianza o narración.
- Nuevo framework E2E, corredor de tests para el frontend, paginación o virtualización de tablas, modo oscuro, kit de componentes completo, pestañas o vistas nuevas, cambios en el orden operativo de los hallazgos.

## Further Notes

- Decisiones tomadas en entrevista previa: fila no entera clicable (evita clics accidentales al seleccionar la acción recomendada), iconos con librería en vez de SVG a mano (mantenimiento), tablas apiladas a ancho completo en el panel, sin pestañas en variables eléctricas.
- El bundle del frontend ya supera el aviso de 500 kB por Recharts; `lucide-react` entra por imports nombrados para no agravarlo innecesariamente.
- Término de glosario ya registrado: «Variables que corroboran» en `CONTEXT.md`.
