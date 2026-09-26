# Orden del panel y menú de usuario

Status: ready-for-agent

## Problem Statement

La persona que opera el panel encuentra la acción «Ejecutar análisis IA» y el estado del «Último análisis» al final de la página, lejos de las tarjetas de resumen que les dan contexto. Las tarjetas se ven planas y sin jerarquía. Estando en el panel general, la navegación ofrece un enlace a la página en la que ya se está. El correo de la sesión y el botón de cerrar sesión ocupan la cabecera como elementos sueltos en vez de agruparse en un control de usuario.

## Solution

Reordenar el panel para que la acción y su estado sigan a las tarjetas de resumen, dar a las tarjetas una elevación sutil con iconos en sus encabezados sin cambiar la paleta, ocultar el enlace de la página actual y agrupar la identidad y la salida en un desplegable con icono de usuario que muestra el correo completo y el cierre de sesión al abrirse.

## User Stories

1. Como operador, quiero encontrar «Análisis» y «Último análisis» justo después de las tarjetas de resumen, para actuar sin bajar hasta el final.
2. Como operador, quiero ver el estado del último intento junto a la acción que lo produjo, para no asociarlos a distancia.
3. Como operador, quiero tarjetas con jerarquía visual (elevación e iconos), para distinguir de un vistazo resumen, acción, hallazgos y medidores.
4. Como operador, quiero que la navegación no me ofrezca ir a la página en la que ya estoy, para no dudar de dónde me encuentro.
5. Como operador, quiero seguir volviendo al panel desde la ficha y el hallazgo, para retomar la investigación.
6. Como usuario de demo, quiero ver solo mi nombre con un icono en la cabecera, para una cabecera despejada.
7. Como usuario de demo, quiero abrir el desplegable y ver mi correo completo y el cierre de sesión, para confirmar con qué cuenta entré antes de salir.
8. Como operador que usa teclado, quiero abrir y cerrar el desplegable, salir con Escape con retorno de foco y cerrar con clic fuera, para operarlo sin ratón.
9. Como operador, quiero que cerrar sesión desde el desplegable invalide mi sesión como antes, para que la cookie anterior no autorice nada.
10. Como evaluador, quiero que el contrato de la API siga intacto tras el reordenamiento, para que la demo no cambie bajo los pies.

## Implementation Decisions

- El panel ordena: tarjetas de resumen, rejilla de acción y estado del análisis, tabla de hallazgos y tabla de medidores; el estado del último intento solo aparece cuando existe un intento.
- Las tarjetas ganan elevación sutil e iconos pequeños en sus encabezados, manteniendo la paleta y la tipografía existentes; la mejora es de superficie, no un sistema de diseño nuevo.
- El enlace de navegación solo se renderiza fuera de la página a la que apunta, de modo que en la ficha y el hallazgo sigue sirviendo de vuelta al panel.
- El control de usuario muestra el icono con la parte local del correo y al abrirse presenta el correo completo en tono tenue más la acción de cerrar sesión con icono; el flujo de cierre (limpieza de consultas y redirección al acceso) no cambia.
- El desplegable es un botón con estado expuesto que cierra con Escape devolviendo el foco y con clic fuera; la sesión sigue verificándose con la misma consulta de sesión.
- Sin cambios de backend, esquema, API ni lógica de análisis: todo el trabajo vive en la interfaz.

## Testing Decisions

- Una buena prueba verifica comportamiento observable (orden de secciones, ausencia del enlace en el panel y presencia fuera, apertura y cierre del desplegable por teclado y ratón, cierre de sesión real, español visible), no detalles de implementación (valores de sombra, componente de icono concreto).
- Costuras, de mayor a menor nivel: recorrido manual de navegador en español como costura principal; compilación y lint del frontend como red de tipos; smoke de solo lectura para confirmar que el contrato no cambió.
- Prior art: el smoke autenticado del contrato API y el recorrido de demo documentado; no se introduce un framework E2E nuevo.

## Out of Scope

- Cambios de backend, esquema, API, detector, confianza o narración.
- Nuevo framework E2E, corredor de tests para el frontend, gestión de cuentas, cambio de credenciales, modo oscuro o nuevas vistas.

## Further Notes

- Decisiones tomadas en entrevista previa: bloque completo de acción y estado tras las tarjetas (no solo la acción); regla general de ocultar el enlace actual en vez de un caso especial para el panel; disparador con parte local del correo porque las cuentas solo tienen email.
