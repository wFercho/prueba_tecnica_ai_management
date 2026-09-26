# Arranque demo en un comando y README documentado

Status: ready-for-agent

## Problem Statement

La persona que evalúa la demo debe encadenar a mano levantar la pila e importar los datos, sin una vía rápida de un solo comando. El README no explica que el análisis puede lanzarse desde la terminal además de desde el botón de la interfaz, ni muestra con diagramas cómo se relacionan el navegador, la API y la base de datos ni el orden del recorrido de demostración.

## Solution

Ofrecer un comando único que levanta la pila e importa los datos sin lanzar el análisis, documentar en el README la vía rápida junto a los comandos uno a uno, dejar por escrito la equivalencia entre el análisis por terminal y el botón de la interfaz, y acompañar con un diagrama de componentes y uno de secuencia del recorrido demo.

## User Stories

1. Como evaluador, quiero arrancar la demo con un solo comando, para no encadenar pasos a mano.
2. Como evaluador, quiero seguir pudiendo levantar e importar por separado, para controlar cada paso cuando lo necesite.
3. Como evaluador, quiero que el comando único no lance el análisis, para mostrar el primer análisis en la interfaz.
4. Como operador, quiero saber que el análisis por terminal y el botón ejecutan la misma lógica, para elegir la vía según el momento.
5. Como operador, quiero ver en un diagrama qué piezas hablan con qué (navegador, API, base de datos y jobs), para ubicar cada acción.
6. Como operador, quiero ver en un diagrama de secuencia el orden del recorrido (arranque, importación, login, análisis, polling, narración, salida), para anticipar lo que va a pasar.
7. Como evaluador, quiero que la demo quede limpia tras verificar el comando único, para mostrar el primer análisis después.

## Implementation Decisions

- El comando único compone levantar la pila e importar los datos, sin análisis; reutiliza los targets existentes en vez de duplicar su lógica.
- El README presenta la vía rápida primero y los comandos uno a uno como control fino, en la sección de demostración existente.
- La nota de equivalencia deja claro que la terminal y el botón invocan la misma ruta de análisis y que el botón sigue siendo la vía principal de la demo.
- El diagrama de componentes centra navegador, API y base de datos con los jobs de importación y provisión al margen; las utilidades de pruebas quedan fuera por no ser runtime.
- El diagrama de secuencia cubre el recorrido demo completo hasta la salida; ambos diagramas viven en línea en el README para que el documento de entrega sea autocontenido.
- Sin cambios de comportamiento, esquema, API ni lógica de análisis.

## Testing Decisions

- Una buena prueba verifica comportamiento observable (el comando levanta e importa, la ayuda lo lista, la demo queda limpia, los diagramas describen el sistema real), no detalles de implementación (redacción exacta del README, sintaxis interna de los diagramas más allá de su validez).
- Costuras, de mayor a menor nivel: ejecución real del comando único seguida de smoke de solo lectura con la demo limpia; listado de ayuda y simulación del target; revisión por inspección de los diagramas.
- Prior art: el smoke autenticado del contrato API y el recorrido de demo documentado; no se introduce nada nuevo.

## Out of Scope

- Cambios de backend, esquema, API, detector, confianza, narración o interfaz.
- Nuevo framework E2E, corredor de tests, renderizado automático de diagramas o nuevas vistas.

## Further Notes

- Decisiones tomadas en entrevista previa: componer en vez de duplicar; no incluir el análisis en el comando único para no romper la demo; diagramas en línea en el README.
