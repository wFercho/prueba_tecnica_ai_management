import { Link } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type AnomalyStatus } from '../api'
import { ConsumptionChart } from '../components/ConsumptionChart'
import {
  ConfidenceBadge,
  SeverityBadge,
  StatusBadge,
  TypeBadge,
} from '../components/badges'
import { formatDateTime, formatKWh, formatNumber, formatPercent } from '../components/format'

const NEXT_STATUSES: { status: AnomalyStatus; label: string }[] = [
  { status: 'ACKNOWLEDGED', label: 'Reconocer' },
  { status: 'RESOLVED', label: 'Marcar como resuelta' },
  { status: 'DISMISSED', label: 'Descartar' },
]

export function AnomalyView({ anomalyId }: { anomalyId: number }) {
  const queryClient = useQueryClient()
  const anomalyQuery = useQuery({
    queryKey: ['anomaly', anomalyId],
    queryFn: () => api.anomaly(anomalyId),
    enabled: Number.isSafeInteger(anomalyId) && anomalyId > 0,
  })
  const change = useMutation({
    mutationFn: (status: AnomalyStatus) => api.setStatus(anomalyId, status),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['anomaly', anomalyId] }),
        queryClient.invalidateQueries({ queryKey: ['summary'] }),
        queryClient.invalidateQueries({ queryKey: ['anomalies'] }),
        queryClient.invalidateQueries({ queryKey: ['meter'] }),
      ])
    },
  })

  if (!Number.isSafeInteger(anomalyId) || anomalyId <= 0) return <p className="notice error" role="alert">El identificador del hallazgo no es válido.</p>
  if (anomalyQuery.isPending) return <p className="empty" role="status">Cargando el hallazgo {anomalyId}…</p>
  if (anomalyQuery.isError) return <p className="notice error" role="alert">No se pudo cargar el hallazgo. <button type="button" onClick={() => void anomalyQuery.refetch()}>Reintentar</button></p>

  const { anomaly } = anomalyQuery.data

  const basis = anomaly.confidence_basis
  const series = anomaly.deviation_series

  return (
    <div>
      <nav aria-label="Ruta de navegación" className="mb-4 text-sm">
        <Link to="/" className="text-blue-700 underline">Panel general</Link>
        <span aria-hidden="true"> / </span>
        <Link to="/medidores/$meterId" params={{ meterId: anomaly.meter_id }} className="text-blue-700 underline">{anomaly.meter_id}</Link>
        <span aria-hidden="true"> / </span><span aria-current="page">Hallazgo {anomaly.id}</span>
      </nav>
      <div className="grid split">
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div className="card">
          <h2>
            {anomaly.meter_name} · {formatDateTime(anomaly.window_start)} →{' '}
            {formatDateTime(anomaly.window_end)}
          </h2>
          {series.length > 0 ? (
            <ConsumptionChart
              points={series.map((point) => ({
                at: new Date(point.timestamp).getTime(),
                value: point.actual_kwh ?? 0,
                baseline: point.baseline_kwh ?? null,
                inAnomaly: true,
              }))}
              height={200}
              label="Lecturas horarias del episodio frente al baseline"
            />
          ) : (
            <p className="empty">No hay lecturas horarias para este hallazgo.</p>
          )}
          <p className="muted">
            La serie corresponde a los datos usados por el detector al analizar el episodio.
          </p>
        </div>

        <div className="card">
          <h2>Resultado de las reglas</h2>
          <p className="prose">
            <span className="prose-label">Explicación</span>
            {anomaly.reason}
          </p>
          <p className="prose">
            <span className="prose-label">Acción recomendada</span>
            {anomaly.recommended_action}
          </p>
          <p className="muted">
            Redactado por {anomaly.explanation_source === 'llm' ? 'el modelo' : 'las reglas'}{' '}
            {anomaly.explanation_status === 'PENDING' && '(en preparación)'}
            {anomaly.explanation_status === 'FAILED' &&
              '(se conserva la versión de las reglas)'}.
          </p>
        </div>

        {change.isError && <p className="notice error" role="alert">No se pudo guardar la decisión.</p>}

        <div className="card">
          <h2>Decisión</h2>
          <p className="muted" style={{ marginTop: 0 }}>
            Esta decisión pertenece al operador; el detector no la modifica.
          </p>
          <div className="actions">
            {NEXT_STATUSES.map((next) => (
              <button
                key={next.status}
                type="button"
                className="action"
                disabled={change.isPending || anomaly.status === next.status}
                onClick={() => change.mutate(next.status)}
              >
                {next.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
        <div className="card">
          <h2>Veredicto</h2>
          <p style={{ margin: '0 0 10px', display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            <TypeBadge type={anomaly.type} />
            <SeverityBadge severity={anomaly.severity} />
            <ConfidenceBadge
              band={anomaly.confidence_band}
              confidence={anomaly.confidence}
            />
            <StatusBadge status={anomaly.status} />
          </p>
          <dl className="facts">
            <dt>Consumo</dt>
            <dd>{formatKWh(anomaly.actual_kwh)}</dd>
            <dt>Baseline</dt>
            <dd>{formatKWh(anomaly.baseline_kwh)}</dd>
            <dt>Desviación</dt>
            <dd>{formatPercent(anomaly.deviation_percent * 100)}</dd>
            <dt>Lecturas afectadas</dt>
            <dd>{anomaly.affected_readings}</dd>
            <dt>Regla</dt>
            <dd className="mono">{anomaly.detected_by}</dd>
            <dt>Medidor</dt>
            <dd>
              <Link to="/medidores/$meterId" params={{ meterId: anomaly.meter_id }} className="text-blue-700 underline">{anomaly.meter_id} →</Link>
            </dd>
          </dl>
        </div>

        <div className="card">
          <h2>Fundamento de la confianza</h2>
          <p className="muted" style={{ marginTop: 0 }}>
            Puntuación de evidencia, no una probabilidad. Cuatro términos ponderados.
          </p>
          <dl className="facts">
            <dt>Desviación</dt>
            <dd>
              {basis.deviation.toFixed(2)} <span className="muted">× 0.40</span>
            </dd>
            <dt>Coincidencia de evento</dt>
            <dd>
              {basis.event_match.toFixed(2)} <span className="muted">× 0.20</span>
            </dd>
            <dt>Corroboración</dt>
            <dd>
              {basis.corroboration.toFixed(2)} <span className="muted">× 0.25</span>
            </dd>
            <dt>Persistencia</dt>
            <dd>
              {basis.persistence.toFixed(2)} <span className="muted">× 0.15</span>
            </dd>
            <dt>Puntuación</dt>
            <dd>{formatNumber(anomaly.confidence, 2)} · {anomaly.confidence_band}</dd>
          </dl>
        </div>

        <div className="card">
          <h2>Corroboración</h2>
          {anomaly.corroborating.length === 0 ? (
            <p className="empty">No se detectaron variables adicionales relacionadas.</p>
          ) : (
            <p style={{ margin: 0 }}>
              {anomaly.corroborating.map((item) => (
                <span key={item} className="badge low" style={{ marginRight: 6 }}>
                  {item}
                </span>
              ))}
            </p>
          )}
        </div>

        <div className="card">
          <h2>Reporte en el periodo</h2>
          {anomaly.correlated_event ? (
            <>
              <p className="prose">
                <span className="prose-label">
                  {anomaly.correlated_event.type} ·{' '}
                  {formatDateTime(anomaly.correlated_event.timestamp)}
                </span>
                {anomaly.correlated_event.description}
              </p>
              <p className="muted" style={{ marginBottom: 0 }}>
                {anomaly.correlated_event.explains
                  ? 'Este evento explica la desviación.'
                  : 'Ningún reporte explica esta desviación.'}
              </p>
            </>
          ) : (
            <p className="empty">No hay reportes en este periodo.</p>
          )}
        </div>

        {anomaly.findings && anomaly.findings.length > 0 && (
          <div className="card">
            <h2>Hallazgos de calidad de datos</h2>
            <table>
              <tbody>
                {anomaly.findings.map((finding) => (
                  <tr key={finding.code}>
                    <td className="mono">{finding.code}</td>
                    <td>{finding.description}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p className="muted" style={{ marginBottom: 0 }}>
              {formatNumber(anomaly.affected_readings)} lecturas requieren validación.
            </p>
          </div>
        )}
      </div>
      </div>
    </div>
  )
}
