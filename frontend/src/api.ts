// The API client. Every route is relative, because the API serves this dashboard
// from the same origin: in production there is nothing to configure, and in
// development Vite proxies the same paths.

export type Severity = 'HIGH' | 'MEDIUM' | 'LOW'
export type ConfidenceBand = 'HIGH' | 'MEDIUM' | 'LOW'
export type AnomalyType =
  | 'REAL_ANOMALY'
  | 'EXPLAINABLE'
  | 'FALSE_POSITIVE'
  | 'DATA_QUALITY'
export type AnomalyStatus = 'OPEN' | 'ACKNOWLEDGED' | 'RESOLVED' | 'DISMISSED'
export type Health = 'HEALTHY' | 'ALERT' | 'CRITICAL'
export type RunState = 'RUNNING' | 'COMPLETED' | 'FAILED'
export type ExplanationStatus = 'PENDING' | 'READY' | 'FAILED'

export interface Meter {
  meter_id: string
  name: string
  location: string
  health: Health
  total_kwh: number
  readings: number
  open_anomalies: number
}

export interface Run {
  id: number
  started_at: string
  finished_at: string | null
  state: RunState
  anomaly_count: number
  window_start: string
  window_end: string
  narrating?: number
  explained?: number
  failed?: number
  error?: string
}

export interface EventSummary {
  timestamp: string
  type: string
  description: string
  explains?: boolean
}

export interface Event {
  id: number
  meter_id: string
  timestamp: string
  type: string
  description: string
}

export interface Anomaly {
  id: number
  run_id: number
  meter_id: string
  meter_name: string
  meter_location: string
  window_start: string
  window_end: string
  affected_readings: number
  type: AnomalyType
  severity: Severity
  confidence: number
  confidence_band: ConfidenceBand
  confidence_basis: {
    deviation: number
    event_match: number
    corroboration: number
    persistence: number
  }
  deviation_percent: number
  actual_kwh: number
  baseline_kwh: number
  corroborating: string[]
  deviation_series: Point[]
  findings: DataQualityFinding[] | null
  correlated_event: EventSummary | null
  reason: string
  recommended_action: string
  detected_by: string
  explanation_source: 'rules' | 'llm'
  explanation_status: ExplanationStatus
  status: AnomalyStatus
}

export interface DataQualityFinding {
  code: string
  description: string
  detail?: string
}

export interface Point {
  timestamp: string
  actual_kwh?: number
  baseline_kwh?: number
  deviation?: number
  voltage_v?: number
  current_a?: number
  power_factor?: number
}

export interface BaselineHour {
  hour: number
  expected_kwh: number
  samples: number
}

export interface MeterDetail {
  meter: { meter_id: string; name: string; location: string }
  health: Health
  total_kwh: number
  points: (Point & { consumption_kwh: number; baseline_kwh: number | null; in_anomaly: boolean; anomaly_id: number | null })[]
  baseline: BaselineHour[]
  anomalies: Anomaly[]
  events: Event[]
}

export interface Summary {
  needs_attention: number
  total_kwh: number
  anomaly_counts: Record<string, number>
  last_run: Run | null
  meters: Meter[]
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: { 'content-type': 'application/json', ...init?.headers },
  })
  if (!response.ok) {
    // The API answers errors as {"error":{"code","message"}}. Falling back to the
    // status keeps a proxy's HTML error page from being shown as the reason.
    const body = (await response.json().catch(() => null)) as
      | { error?: { message?: string } }
      | null
    throw new ApiError(response.status, body?.error?.message ?? `the API answered ${response.status}`)
  }
  return (await response.json()) as T
}

export const api = {
  summary: () => request<Summary>('/dashboard/summary'),
  anomalies: () => request<{ anomalies: Anomaly[] }>('/anomalies'),
  anomaly: (id: number) => request<{ anomaly: Anomaly }>(`/anomalies/${id}`),
  meter: (meterId: string) => request<MeterDetail>(`/meters/${encodeURIComponent(meterId)}`),
  setStatus: (id: number, status: AnomalyStatus) =>
    request<{ anomaly: Anomaly }>(`/anomalies/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    }),
  analyze: () => request<{ run: Run }>('/ai/analyze', { method: 'POST' }),
  analysis: (runId: number) => request<{ run: Run }>(`/ai/analysis/${runId}`),
}
