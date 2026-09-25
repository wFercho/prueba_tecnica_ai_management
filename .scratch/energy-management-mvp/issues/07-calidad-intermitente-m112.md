# 07: Investigar la calidad intermitente de M-112

**What to build:** Al abrir M-112, el operador entiende por qué un consumo estable acompaña un episodio `DATA_QUALITY/HIGH`: qué valores eléctricos contradicen al consumo, a qué horas, y por qué el `status=OK` original no los valida.

**Blocked by:** 05 — Ejecutar análisis y ver cuatro episodios inmediatamente.

**Status:** ready-for-agent

- [ ] El veredicto de calidad se fundamenta en robust bounds y consistencia eléctrica; retirar el reporte etiquetado `DATA_QUALITY` no elimina la detección cuando las lecturas físicas siguen siendo ofensivas.
- [ ] Los 16 valores defectuosos, recurrentes cada tres horas dentro de lecturas **horarias**, forman un episodio con contador de 16; lecturas normales intercaladas no se cuentan como afectadas.
- [ ] La API y el detalle exponen hallazgos separados de los cuatro términos de `confidence_basis`, con hora, variable y valores ofensores; el frontend consume la forma real del JSON y la explica en español.
- [ ] El `status=OK` ingerido se conserva verbatim y se distingue del hallazgo derivado. Las descripciones inglesas de reportes se explican en español sin modificar el origen.
- [ ] Pruebas de detector, HTTP y vista comprueban clasificación, evidencia eléctrica y que M-112 no se convierte en anomalía de consumo ni necesita un flag derivado para todas las lecturas.
