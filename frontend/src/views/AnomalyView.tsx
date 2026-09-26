import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { ConsumptionChart } from '../components/ConsumptionChart'
import {
  ConfidenceBadge,
  SeverityBadge,
  StatusBadge,
  TypeBadge,
} from '../components/badges'
import { formatDateTime, formatKWh, formatNumber, formatPercent } from '../components/format'

const variableLabel: Record<string, string> = {
  voltage: 'voltaje (V)', current: 'corriente (A)', power_factor: 'factor de potencia', energy_balance: 'balance energético',
}

const eventLabel: Record<string, string> = {
  OPERATIONAL_CHANGE: 'Cambio operativo', SCHEDULED_OUTAGE: 'Parada programada',
  DATA_QUALITY: 'Reporte de calidad', UNKNOWN: 'Sin evento operativo reportado',
}

export function AnomalyView({ anomalyId }: { anomalyId: number }) {
  const anomalyQuery = useQuery({
    queryKey: ['anomaly', anomalyId],
    queryFn: () => api.anomaly(anomalyId),
    enabled: Number.isSafeInteger(anomalyId) && anomalyId > 0,
    refetchInterval: (query) => query.state.data?.anomaly.explanation_status === 'PENDING' ? 1200 : false,
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
              label="Lecturas horarias del episodio frente a la línea base"
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
            <dd>{formatPercent(anomaly.deviation_percent)}</dd>
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
            <dd>{formatNumber(anomaly.confidence, 2)} · <ConfidenceBadge band={anomaly.confidence_band} confidence={anomaly.confidence} /></dd>
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
                  {eventLabel[anomaly.correlated_event.type] ?? 'Reporte de contexto'} ·{' '}
                  {formatDateTime(anomaly.correlated_event.timestamp)}
                </span>
                {anomaly.correlated_event.type === 'UNKNOWN'
                  ? 'No se comunicó ningún evento que explique el cambio.'
                  : 'Este reporte aporta contexto al episodio.'}
              </p>
              <p className="muted" style={{ marginBottom: anomaly.correlated_event.description ? 8 : 0 }}>
                {anomaly.correlated_event.explains
                  ? 'Este evento explica la desviación.'
                  : 'Ningún reporte explica esta desviación.'}
              </p>
              {anomaly.correlated_event.description && (
                <p className="muted" style={{ marginBottom: 0 }}>
                  Texto original del reporte (fuente, sin traducir):{' '}
                  <span lang="en">“{anomaly.correlated_event.description}”</span>
                </p>
              )}
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
                  <tr key={`${finding.timestamp}-${finding.variable}`}>
                    <td className="mono">{formatDateTime(finding.timestamp)}</td>
                    <td>{variableLabel[finding.variable] ?? finding.variable}: {formatNumber(finding.value, 2)} frente a {formatNumber(finding.expected, 2)} esperados</td>
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
