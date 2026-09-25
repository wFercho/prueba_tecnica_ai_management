# **PRUEBA TÉCNICA** 

## **AI Energy Management Platform** 

Backend + Frontend + Data + Inteligencia Artificial 

Construir un MVP funcional para gestionar medidores eléctricos y utilizar IA para detectar, explicar, priorizar y recomendar acciones sobre anomalías. 

|**Alcance**|**Valores**|
|---|---|
|Medidores|12|
|Periodo|14 días|
|Lecturas|4.032|
|Variables|Consumo, voltaje, corriente, factor de potencia|
|Demo|5–10 minutos|



### **1. Objetivo de la prueba** 

La prueba evalúa la capacidad de construir una solución end-to-end que combine Backend, Frontend, Data/Analytics e Inteligencia Artificial. No se busca únicamente un CRUD de medidores, sino una experiencia que convierta datos en una decisión operativa. 

### **2. Capacidades evaluadas** 

Backend: API, persistencia y procesamiento. Frontend: UX, dashboard, navegación, filtros y gráficas. Data/Analytics: series temporales, baseline, outliers y calidad de datos. IA: detección, explicación, priorización, confianza y recomendación. Engineering: mantenibilidad, testing y documentación. 

La plataforma debe responder: 

- ¿Qué está pasando con los medidores? 

- ¿Qué lecturas se salen de su comportamiento esperado? 

- ¿La anomalía es real, explicable o de calidad de datos? 

- ¿Cuál debería investigarse primero? 

- ¿Por qué la IA llegó a esa conclusión? 

- ¿Qué acción recomienda? 

### **3. Flujo principal** 

```
Dashboard → Medidores → Detalle → Anomalías IA → Investigación → Acción
```

### **5. Dashboard** 

|**KPI**|**Por ejemplo**|
|---|---|
|Medidores|12|
|Consumo|Consumo total del periodo|
|Anomalías IA|4 detectadas|
|Alta prioridad|2|
|Confianza IA|Métrica agregada|
|Último análisis|Fecha/hora y estado|



### **<u>6. Gestión de medidores</u>** 

|**Medidor**|**Consumo Variación**|**Estado**|**Nivel**|
|---|---|---|---|
|M-101|820 kWh +2,5%|OK|—|
|M-104|1.860 kWh +47,6%|Alert|Medium|
|M-109|2.180 kWh +103,7%|Critical|High|
|M-112|690 kWh -1,4%|Alert|High|



- Filtros: todos, normales, alertas y críticas. 

- Búsqueda por meter_id. 

- Ordenamiento por consumo, variación o severidad. 

### **7. Detalle de medidor** 

Debe mostrar consumo actual, baseline, variación, estado e histórico. Idealmente también voltaje, corriente y factor de potencia. 

Ejemplo: M-109 · 2.180 kWh · baseline aproximado 1.070 kWh · +103,7%. 

### **8. Motor de anomalías** 

La técnica es libre: reglas, estadística, Z-score, Isolation Forest, series de tiempo, ML, LLM o combinación. Se evalúa el resultado, no una tecnología específica. 

- Spikes/cambios bruscos. 

- Cambios persistentes respecto al baseline. 

- Outliers. 

- Patrones horarios anormales. 

- Problemas de calidad de datos. 

- Relaciones anómalas entre consumo, voltaje, corriente y factor de potencia. 

- Falsos positivos cuando un evento operativo explica el cambio. 

### **<u>9. Casos del dataset</u>** 

|**Medidor**|**Caso**|**Resultado esperado**|
|---|---|---|
|M-104|Aumento coincidente con nueva<br>línea productiva.|Anomalía explicable / Medium|
|M-106|Cambio explicado por parada<br>programada.|False positive / Low|
|M-109|Aumento >100% sin evento<br>conocido y con cambios eléctricos.|Anomalía real / High|
|M-112|Consumo estable pero lecturas<br>eléctricas inconsistentes.|Data quality / High|



### **10. Uso de Inteligencia Artificial** 

La IA no debe limitarse a true/false. Debe generar explicación y recomendación sustentadas por los datos. 

```
{
"meter_id": "M-109",
"anomaly": true,
"type": "REAL_ANOMALY",
"severity": "HIGH",
"confidence": 0.96,
"reason": "Consumo 103,7% por encima del baseline sin evento conocido.",
"recommended_action": "Investigar medidor e instalación."
}
```

### **11. Pantalla de Anomalías IA** 

**<u><mark>Medidor Tipo Severidad Confianza A</mark> cción</u>** 

|M-109|Real anomaly High|Alta|Investigar|
|---|---|---|---|
|M-112|Data quality High|Alta|Validar|
|M-104|Explainable anomaly Medium|Alta|Validar operación|
|M-106|False positive Low|Media/Alta|No escalar|



### **12. Investigación** 

- Qué encontró la IA. 

- Variables que cambiaron. 

- Comparación contra baseline. 

- Eventos relacionados. 

- Severidad y confianza. 

- Acción recomendada. 

- Evidencia que soporta la explicación. 

### **13. Ejecutar análisis IA** 

```
Lecturas → Baseline → Detección → Correlación → Eventos → Explicación → Recomendación
```

Debe existir un botón Run AI Analysis y mostrar el estado del proceso. Ejemplo final: 4 anomalías detectadas · 2 requieren atención prioritaria. 

### **14. Arquitectura sugerida** 

Usar como lenguaje go 

La arquitectura es libre 

### **15. API mínima sugerida** 

- GET /meters 

- GET /meters/:meterId 

- GET /meters/:meterId/readings 

- GET /anomalies 

- GET /anomalies/:id 

- POST /ai/analyze 

- GET /ai/analysis/:id 

- GET /dashboard/summary 

### **<u>16. Modelo de datos sugerido</u>** 

|**Entidad**|**Campos principales**|
|---|---|
|Meter|id, meter_id, name, location, status, created_at|
|Reading|id, meter_id, timestamp, consumption_kwh,<br>voltage_v, current_a, power_factor, status|
|Event|id, meter_id, timestamp, type, description|
|Anomaly|id, meter_id, detected_at, type, severity, confidence,<br>reason, recommended_action, status|



### **<u>17. Evaluación global</u>** 

|**Competencia**|**Score**|
|---|---|
|Frontend / UX|20|
|Backend / API|20|
|Data / Analytics|20|
|Detección de anomalías|15|
|IA y explicabilidad|15|



Testing / documentación / calidad 

10 

### **<u>18. Evaluación específca de IA  i</u>** 

|**Criterio**|**Score**|
|---|---|
|Detecta M-109|30|
|Prioriza M-109|25|
|Evita tratar M-106 como anomalía real|15|
|Detecta M-112 como problema de calidad|10|
|Explica con evidencia|10|
|Recomienda acción coherente|10|



### **<u>19. Datos entregados</u>** 

|**Archivo**|**Uso**|**Candidato**|
|---|---|---|
|readings.csv|4.032 lecturas de 12 medidores<br>durante 14 días.|Sí|
|events.csv|Eventos operativos conocidos.|Sí|
|expected_results.csv|Ground truth para evaluación.|NO; reservado al evaluador|



Importante: expected_results.csv no debe estar disponible para el modelo ni para el usuario final. Se utiliza únicamente para evaluar el desempeño. 

### **20. Entregables** 

- Repositorio Git. 

- Frontend funcional. 

- Backend funcional. 

- Demo de 5–10 minutos. 

### **21. UX y demo** 

La aplicación debe sentirse como un producto SaaS de Energy Management y no como una colección de pantallas de prueba. 

```
Login → Dashboard → M-109 → Run AI Analysis → Anomalía → Explicación → Acción
```

El evaluador debe poder entender en pocos minutos qué aporta la IA. 

### **24. Criterio principal de éxito** 

La solución debe demostrar el ciclo completo: DATOS → ANÁLISIS → ANOMALÍA → EXPLICACIÓN → PRIORIZACIÓN → ACCIÓN. El valor principal está en que una persona pueda identificar rápidamente qué requiere atención y entender por qué. 

**Archivos: readings.csv · events.csv · expected_results.csv** 

