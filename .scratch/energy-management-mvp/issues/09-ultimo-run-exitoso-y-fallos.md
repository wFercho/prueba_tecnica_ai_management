# 09: Conservar el último run exitoso cuando otro falla

**What to build:** El operador puede repetir el análisis sin duplicar hallazgos visibles y, si un intento posterior falla, seguir consultando los últimos resultados fiables con un aviso explícito de que son anteriores al fallo.

**Blocked by:** 05 — Ejecutar análisis y ver cuatro episodios inmediatamente.

**Status:** resolved

- [ ] Dos ejecuciones correctas del mismo dataset conservan dos runs atribuibles pero el panel, la tabla y las fichas muestran solo los cuatro episodios del último run completado, nunca ocho ni una mezcla.
- [ ] Un intento que falla antes de persistir anomalías queda registrado como `FAILED`; el estado/fecha del intento reciente se muestran separados de la fecha y los resultados del último run exitoso.
- [ ] Si existe un run exitoso anterior, las anomalías permanecen visibles con aviso «Resultados anteriores: el último análisis falló»; si nunca hubo éxito, se presenta el error y ninguna anomalía inventada.
- [ ] Una explicación que llega tarde a un run anterior no altera el conjunto visible del run más reciente.
- [ ] Tests HTTP con narrador controlado y fallo de almacenamiento, más integración PostgreSQL del historial, comprueban atribución, selección del último exitoso y consultas de detalle consistentes.
