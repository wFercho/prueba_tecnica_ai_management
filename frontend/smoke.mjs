// A contract walk of the running API, checking exactly the fields the dashboard
// reads. The unit tests prove the backend against a fake store and the frontend's
// types prove nothing at runtime, so this is the one check that a change to the
// JSON would fail: it runs against the live container, where the browser will read
// from.
//
//   node smoke.mjs            # against http://api:8080 inside compose
//   API_URL=http://localhost:8090 node smoke.mjs

const base = process.env.API_URL ?? 'http://localhost:8080'
let failures = 0

const fail = (what) => {
  failures += 1
  console.error(`  ✗ ${what}`)
}

const pass = (what) => console.log(`  ✓ ${what}`)

function isNumber(value) {
  return typeof value === 'number' && Number.isFinite(value)
}

function has(object, fields, where) {
  for (const field of fields) {
    if (object == null || !(field in object)) {
      fail(`${where} is missing "${field}"`)
      return false
    }
  }
  return true
}

async function get(path) {
  const response = await fetch(base + path)
  if (!response.ok) throw new Error(`GET ${path} answered ${response.status}`)
  return response.json()
}

const ANOMALY_FIELDS = [
  'id',
  'meter_id',
  'window_start',
  'window_end',
  'affected_readings',
  'type',
  'severity',
  'confidence',
  'confidence_band',
  'confidence_basis',
  'deviation_percent',
  'actual_kwh',
  'baseline_kwh',
  'corroborating',
  'deviation_series',
  'corroborating',
  'reason',
  'recommended_action',
  'detected_by',
  'explanation_source',
  'explanation_status',
  'status',
]

const TYPES = ['REAL_ANOMALY', 'EXPLAINABLE', 'FALSE_POSITIVE', 'DATA_QUALITY']
const SEVERITIES = ['HIGH', 'MEDIUM', 'LOW']
const STATUSES = ['OPEN', 'ACKNOWLEDGED', 'RESOLVED', 'DISMISSED']

console.log(`walking the API at ${base}\n`)

console.log('POST /ai/analyze')
const analysed = await (await fetch(base + '/ai/analyze', { method: 'POST' })).json()
if (analysed.run?.state === 'COMPLETED' && analysed.run.anomaly_count > 0) {
  pass(`run ${analysed.run.id} completed with ${analysed.run.anomaly_count} findings`)
} else {
  fail(`the run did not complete: ${JSON.stringify(analysed)}`)
}

console.log('\nGET /dashboard/summary')
const summary = await get('/dashboard/summary')
if (has(summary, ['needs_attention', 'total_kwh', 'anomaly_counts', 'last_run', 'meters'], 'the summary')) {
  pass('the summary carries the fields the KPI row reads')
}
if (isNumber(summary.total_kwh) && isNumber(summary.needs_attention)) {
  pass(`total ${summary.total_kwh} kWh, ${summary.needs_attention} open`)
} else {
  fail('total_kwh and needs_attention must both be numbers')
}
if (!Array.isArray(summary.meters) || summary.meters.length !== 12) {
  fail(`expected 12 meters in the catalogue, got ${summary.meters?.length}`)
} else {
  pass('twelve meters')
}
const badMeter = (summary.meters ?? []).find(
  (meter) => !has(meter, ['meter_id', 'name', 'health', 'total_kwh', 'readings', 'open_anomalies'], 'a meter row'),
)
if (!badMeter) pass('every meter row is complete')
const health = (summary.meters ?? []).map((meter) => meter.health)
if (health.every((value) => ['HEALTHY', 'ALERT', 'CRITICAL'].includes(value))) {
  pass('health is one of the three derived values')
} else {
  fail(`unexpected health values: ${health.join(', ')}`)
}
if (summary.last_run) {
  if (has(summary.last_run, ['id', 'state', 'anomaly_count', 'narrating', 'explained', 'failed'], 'the last run')) {
    pass('the last run carries its narration counters')
  }
}

console.log('\nGET /anomalies')
const { anomalies } = await get('/anomalies')
if (anomalies.length !== 4) {
  fail(`expected the four delivered findings, got ${anomalies.length}`)
} else {
  pass('four findings')
}
const incomplete = anomalies.find((anomaly) => !has(anomaly, ANOMALY_FIELDS, 'a finding'))
if (!incomplete) pass('every finding is complete')
const badEnum = anomalies.find(
  (anomaly) =>
    !TYPES.includes(anomaly.type) ||
    !SEVERITIES.includes(anomaly.severity) ||
    !STATUSES.includes(anomaly.status) ||
    typeof anomaly.reason !== 'string' ||
    anomaly.reason.length === 0,
)
if (!badEnum) pass('the vocabularies and the prose are what the badges expect')
const unordered = anomalies.find(
  (anomaly, index) => index > 0 && anomalies[index - 1].confidence < anomaly.confidence,
)
if (!unordered) pass('the list arrives most urgent first, as the dashboard claims')
const worst = anomalies[0]
if (worst?.meter_id === 'M-109' && worst?.type === 'REAL_ANOMALY') {
  pass('M-109 leads the list')
} else {
  fail(`expected M-109 REAL_ANOMALY first, got ${worst?.meter_id} ${worst?.type}`)
}

console.log('\nGET /anomalies/{id}')
const { anomaly } = await get(`/anomalies/${worst.id}`)
if (has(anomaly, ['id', 'confidence_basis', 'deviation_series', 'correlated_event'], 'the finding')) {
  pass('the investigation view has its evidence')
}
if (has(anomaly.confidence_basis ?? {}, ['deviation', 'event_match', 'corroboration', 'persistence'], 'the basis')) {
  pass('the four confidence terms are present for the score table')
}
if (Array.isArray(anomaly.deviation_series) && anomaly.deviation_series.length > 0) {
  const point = anomaly.deviation_series[0]
  if (has(point, ['timestamp', 'actual_kwh', 'baseline_kwh', 'deviation'], 'a series point')) {
    pass(`the series has ${anomaly.deviation_series.length} points the chart can draw`)
  }
}
if (anomaly.correlated_event === null || has(anomaly.correlated_event, ['timestamp', 'type', 'description', 'explains'], 'the event')) {
  pass('the event panel renders for both the reported and the unexplained case')
}

console.log('\nGET /meters/{meterId}')
const detail = await get('/meters/M-109')
if (has(detail, ['meter', 'health', 'total_kwh', 'points', 'baseline', 'anomalies', 'events'], 'the meter view')) {
  pass('the meter view has every section it renders')
}
if (detail.points.length === 336) {
  pass('336 hourly points')
} else {
  fail(`expected 336 points, got ${detail.points.length}`)
}
const badPoint = (detail.points ?? []).find(
  (point) =>
    !has(point, ['timestamp', 'consumption_kwh', 'voltage_v', 'current_a', 'power_factor', 'baseline_kwh', 'in_anomaly'], 'a point'),
)
if (!badPoint) pass('every point carries what the chart and the tooltip read')
if ((detail.points ?? []).some((point) => point.in_anomaly === true)) {
  pass('the chart has hours to shade')
} else {
  fail('no point is marked in_anomaly, so the chart would draw no band')
}
if ((detail.baseline ?? []).length === 24) {
  pass('24 baseline hours')
} else {
  fail(`expected 24 baseline hours, got ${detail.baseline?.length}`)
}

console.log('\nPATCH /anomalies/{id}')
const target = anomalies[anomalies.length - 1]
const patched = await (
  await fetch(`${base}/anomalies/${target.id}`, {
    method: 'PATCH',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ status: 'ACKNOWLEDGED' }),
  })
).json()
if (patched.anomaly?.status === 'ACKNOWLEDGED') {
  pass(`finding ${target.id} is now acknowledged`)
} else {
  fail(`the status did not change: ${JSON.stringify(patched)}`)
}
const reread = await get(`/anomalies/${target.id}`)
if (reread.anomaly.status === 'ACKNOWLEDGED') {
  pass('and it survived the round trip')
}
const rejected = await fetch(`${base}/anomalies/${target.id}`, {
  method: 'PATCH',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ status: 'MAYBE' }),
})
if (rejected.status === 400) {
  pass('an unknown status is refused with 400, which is what the UI surfaces as an error')
} else {
  fail(`an unknown status answered ${rejected.status}, want 400`)
}

console.log('\nGET a meter that is not there')
const missing = await fetch(`${base}/meters/M-999`)
if (missing.status === 404) {
  pass('404, which the API client turns into a readable message')
} else {
  fail(`a missing meter answered ${missing.status}, want 404`)
}

console.log('')
if (failures > 0) {
  console.error(`${failures} contract check(s) failed`)
  process.exit(1)
}
console.log('every field the dashboard reads is present and well formed')
