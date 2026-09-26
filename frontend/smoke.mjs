// A read-only contract check by default. It does NOT trigger the first analysis
// in a database reserved for showing that action during a live demonstration.
// To run the full four-episode contract on an isolated stack (or after the demo),
// set SMOKE_ANALYZE=1; otherwise the check observes whichever state exists.
const base = process.env.API_URL ?? 'http://localhost:8080'
const password = process.env.DEMO_PASSWORD ?? 'admin'
let failures = 0
const check = (ok, message) => {
  if (ok) console.log(`  ✓ ${message}`)
  else { console.error(`  ✗ ${message}`); failures++ }
}

const login = await fetch(base + '/auth/login', {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ email: 'admin@email.com', password }),
})
check(login.status === 200, 'la cuenta de demostración permite iniciar sesión')
if (!login.ok) process.exit(1)
const cookie = login.headers.get('set-cookie')?.split(';')[0]
check(Boolean(cookie?.startsWith('energy_session=')), 'la API devuelve una cookie de sesión')
const request = (path, init = {}) => fetch(base + path, {
  ...init, headers: { cookie, ...init.headers },
})
const get = async (path) => {
  const response = await request(path)
  if (!response.ok) throw new Error(`GET ${path}: ${response.status}`)
  return response.json()
}

if (process.env.SMOKE_ANALYZE === '1') {
  const response = await request('/ai/analyze', { method: 'POST' })
  const body = await response.json()
  check(response.status === 202 && body.run?.anomaly_count === 4, 'el análisis devuelve 202 tras persistir cuatro episodios')
}

const summary = await get('/dashboard/summary')
check(summary.meters?.length === 12, 'el catálogo contiene 12 medidores')
check(typeof summary.total_kwh === 'number', 'el resumen contiene consumo del periodo')
check('last_successful_run' in summary && 'high_priority' in summary && 'confidence_bands' in summary,
  'el resumen separa último éxito, prioridad y distribución de confianza')
const { anomalies } = await get('/anomalies')

if (!summary.last_successful_run) {
  check(anomalies.length === 0, 'antes del primer análisis no hay resultados precocinados')
  check(summary.meters.every((meter) => meter.health === 'UNASSESSED' || meter.health === 'INSUFFICIENT_DATA'),
    'antes del primer análisis ningún medidor se declara sano')
} else {
  check(anomalies.length === 4, 'cuatro episodios visibles, no acumulados de runs anteriores')
  check(summary.high_priority === 2, 'dos episodios de prioridad alta')
  check(anomalies[0]?.meter_id === 'M-109', 'M-109 encabeza la lista')
  check(anomalies.every((item) => item.anomaly === true && item.status === 'OPEN' &&
    item.reason?.length > 0 && item.recommended_action?.length > 0),
  'todas las filas tienen veredicto, estado abierto y texto por reglas')
  const m112 = anomalies.find((item) => item.meter_id === 'M-112')
  const row112 = summary.meters.find((item) => item.meter_id === 'M-112')
  check(m112?.type === 'DATA_QUALITY' && m112.severity === 'HIGH' && m112.affected_readings === 16 && row112?.health === 'ALERT',
    'M-112 conserva calidad HIGH y salud ALERT con 16 lecturas afectadas')
  if (m112) {
    const detail112 = await get(`/anomalies/${m112.id}`)
    check(detail112.anomaly.findings?.some((item) => item.timestamp && item.variable === 'voltage' && typeof item.value === 'number'),
      'el detalle contiene los valores y horas ofensores de calidad')
    const refused = await request(`/anomalies/${m112.id}`, {
      method: 'PATCH', headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ status: 'ACKNOWLEDGED' }),
    })
    check(refused.status === 405, 'no existe endpoint de workflow para cambiar estado')
  }
  const meter = await get('/meters/M-109')
  check(meter.points?.length === 336 && meter.baseline?.length === 24,
    'M-109 ofrece la serie horaria y su línea base')
  check(meter.points?.filter((point) => point.in_anomaly).length === 58,
    'el gráfico destaca exactamente las 58 horas afectadas de M-109')
}

const loggedOut = await request('/auth/logout', { method: 'POST' })
check(loggedOut.status === 204, 'se puede cerrar sesión')
const revoked = await request('/dashboard/summary')
check(revoked.status === 401, 'la cookie revocada ya no autoriza consultas')

if (failures) process.exit(1)
console.log('Contrato del panel y autenticación comprobados')
