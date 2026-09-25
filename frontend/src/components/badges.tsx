import type { AnomalyStatus, ConfidenceBand, Health, Run, Severity } from '../api'
import { formatNumber } from './format'

const SEVERITY_CLASS: Record<Severity, string> = {
  HIGH: 'high',
  MEDIUM: 'medium',
  LOW: 'low',
}

// Health is deliberately not Severity's vocabulary, so a meter's standing can never
// be read interchangeably with a finding's urgency.
const HEALTH_CLASS: Record<Health, string> = {
  CRITICAL: 'high',
  ALERT: 'medium',
  HEALTHY: 'low',
}

const TYPE_LABEL: Record<string, string> = {
  REAL_ANOMALY: 'Anomalía real',
  EXPLAINABLE: 'Explicable',
  FALSE_POSITIVE: 'Falso positivo',
  DATA_QUALITY: 'Calidad de datos',
}

const BAND_LABEL: Record<Severity, string> = { HIGH: 'Alta', MEDIUM: 'Media', LOW: 'Baja' }
const HEALTH_LABEL: Record<Health, string> = { CRITICAL: 'Crítico', ALERT: 'Alerta', HEALTHY: 'Normal' }
const STATUS_LABEL: Record<AnomalyStatus, string> = { OPEN: 'Abierta', ACKNOWLEDGED: 'Reconocida', RESOLVED: 'Resuelta', DISMISSED: 'Descartada' }

export function SeverityBadge({ severity }: { severity: Severity }) {
  return <span className={`badge ${SEVERITY_CLASS[severity]}`}>{BAND_LABEL[severity]}</span>
}

export function HealthBadge({ health }: { health: Health }) {
  return <span className={`badge ${HEALTH_CLASS[health]}`}>{HEALTH_LABEL[health]}</span>
}

export function TypeBadge({ type }: { type: string }) {
  return <span className="badge type">{TYPE_LABEL[type] ?? type}</span>
}

export function ConfidenceBadge({
  band,
  confidence,
}: {
  band: ConfidenceBand
  confidence: number
}) {
  return (
    <span
      className={`badge ${SEVERITY_CLASS[band as Severity]}`}
      title="Puntuación de evidencia, no probabilidad"
    >
      Confianza {formatNumber(confidence, 2)} · {BAND_LABEL[band]}
    </span>
  )
}

export function StatusBadge({ status }: { status: AnomalyStatus }) {
  const cls = status === 'OPEN' ? '' : status === 'DISMISSED' ? 'muted' : 'low'
  return <span className={`badge ${cls}`}>{STATUS_LABEL[status]}</span>
}

// Confidence is described in words rather than as a percentage of being right,
// because it is not a probability. Showing two decimals keeps the ordering legible
// without implying a precision the score does not have.
export function BandOnly({ band }: { band: ConfidenceBand }) {
  return <span className={`badge ${SEVERITY_CLASS[band]}`}>{BAND_LABEL[band]}</span>
}

export function RunStateBadge({ run }: { run: Run }) {
  if (run.state === 'FAILED') {
    return <span className="badge high">Fallido</span>
  }
  if (run.state === 'RUNNING') {
    return <span className="badge medium">En curso</span>
  }
  const narrating = run.narrating ?? 0
  if (narrating > 0) {
    return <span className="badge medium">Redactando {narrating}</span>
  }
  if ((run.failed ?? 0) > 0) {
    return <span className="badge medium">{run.failed} narraciones fallidas</span>
  }
  return <span className="badge low">Completado</span>
}
