# 06: Gráfico temporal legible de M-109

**What to build:** El operador compara en la ficha M-109 consumo real y línea base sobre 14 días, y solo tras analizar distingue como un bloque las 58 horas del episodio. Puede consultar los valores puntuales sin depender únicamente de color o ratón.

**Blocked by:** 05 — Ejecutar análisis y ver cuatro episodios inmediatamente.

**Status:** resolved

- [ ] Un spike temprano con Recharts verifica los cuatro criterios antes de migrar los demás gráficos: banda diferenciada que no tape el baseline, tooltip español con hora/real/baseline/desviación, eje legible por días y 58 horas como bloque continuo.
- [ ] El gráfico antes del primer análisis muestra lecturas y baseline sin banda; después usa la ventana del episodio realmente persistido y no una marca especial por ID de medidor.
- [ ] Consumo y baseline se distinguen por trazo/etiqueta además del color; teclado y resumen o tabla textual permiten acceder a la evidencia.
- [ ] El detalle muestra consumo y baseline del mismo periodo, última lectura horaria aparte y voltaje/corriente/factor de potencia con unidades y fechas en español.
- [ ] Si Recharts falla un criterio tras el spike acotado, se documenta el fallo y se mantiene SVG manual **solo para este gráfico**; el criterio funcional sigue siendo el mismo.
- [ ] Un recorrido visual y pruebas del contrato de datos cubren estados previo/posterior al run y evitan volver a multiplicar por cien un porcentaje ya expresado como porcentaje.
