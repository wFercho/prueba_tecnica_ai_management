# 04: Reimportar datos sin resultados precalculados

**What to build:** El evaluador puede ejecutar la importación dos veces y abrir el panel y M-109 **antes** del análisis sin ver anomalías pregrabadas ni salud atribuida sin evidencia. Su cuenta y sesión permanecen disponibles durante el reinicio de los datos de demo.

**Blocked by:** 02 — Iniciar sesión con una cuenta de demo persistida.

**Status:** ready-for-agent

- [ ] La importación explícita reemplaza medidores, lecturas y reportes de contexto y elimina runs/anomalías derivados como una operación segura ante fallos; repetirla deja exactamente el mismo dataset de entrada.
- [ ] Usuarios y sesiones no se borran ni se reprovisionan en cada seed; un usuario ya autenticado puede consultar los datos recién importados.
- [ ] Antes del primer run, el panel muestra «Último análisis: nunca» y «Pendiente de análisis» para resultados, y la salud aparece «Sin analizar», no `HEALTHY` ni `0 anomalías` como veredicto.
- [ ] La ficha M-109 deja ver lecturas y baseline, pero no una banda de episodio ni explicación precalculadas.
- [ ] Un test PostgreSQL comprueba rollback e identidades preservadas; tests HTTP/UI comprueban el estado inicial y la ausencia de resultados. No se utiliza ni importa `expected_results.csv`.
