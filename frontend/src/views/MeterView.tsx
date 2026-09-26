import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { ConsumptionChart } from '../components/ConsumptionChart'
import {
  HealthBadge,
  SeverityBadge,
  StatusBadge,
  TypeBadge,
} from '../components/badges'
import { formatDateTime, formatKWh, formatPercent } from '../components/format'

export function MeterView({ meterId }: { meterId: string }) {
  const detailQuery = useQuery({
    queryKey: ['meter', meterId],
    queryFn: () => api.meter(meterId),
    refetchInterval: (query) => query.state.data?.anomalies.some((a) => a.explanation_status === 'PENDING') ? 1200 : false,
  })
  const summaryQuery = useQuery({ queryKey: ['summary'], queryFn: api.summary })

  if (detailQuery.isPending || summaryQuery.isPending) {
    return <p className="empty" role="status">Cargando la ficha de {meterId}…</p>
  }
  if (detailQuery.isError || summaryQuery.isError) {
    return (
      <div className="card" role="alert">
        <p>No se pudo cargar la ficha de {meterId}.</p>
        <button type="button" className="action" onClick={() => { void detailQuery.refetch(); void summaryQuery.refetch() }}>Reintentar</button>
        <Link to="/" className="ml-3 text-blue-700 underline">Volver al panel general</Link>
      </div>
    )
  }

  const detail = detailQuery.data
  const { meter, points, baseline, anomalies, events, health, total_kwh } = detail
  const baselineTotal = points.every((point) => point.baseline_available)
    ? points.reduce((total, point) => total + point.baseline_kwh, 0) : null
  const latest = points.at(-1)

  return (
    <div>
      <nav aria-label="Ruta de navegación" className="mb-4 text-sm">
        <Link to="/" className="text-blue-700 underline underline-offset-2">Panel general</Link>
        <span aria-hidden="true"> / </span><span aria-current="page">Medidor {meter.meter_id}</span>
      </nav>
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start">
      <div className="flex min-w-0 flex-col gap-4">
        <div className="card">
          <h2>Consumo frente a su línea base</h2>
          <ConsumptionChart
            points={points.map((point) => ({
              at: new Date(point.timestamp).getTime(),
              value: point.consumption_kwh,
               baseline: point.baseline_available ? point.baseline_kwh : null,
              inAnomaly: point.in_anomaly,
            }))}
            label={`${meter.meter_id}: consumo horario frente a su línea base`}
          />
          <p className="muted">
            La línea base representa el consumo habitual de este medidor por hora,
            calculado a partir de su propio historial.
          </p>
          <dl className="facts">
            <dt>Consumo del periodo</dt><dd>{formatKWh(total_kwh)}</dd>
            <dt>Línea base del mismo periodo</dt><dd>{baselineTotal == null ? 'Sin datos suficientes' : formatKWh(baselineTotal)}</dd>
            <dt>Última lectura horaria</dt><dd>{latest ? `${formatDateTime(latest.timestamp)} · ${formatKWh(latest.consumption_kwh)}` : 'Sin lecturas'}</dd>
          </dl>
        </div>

        <div className="card">
          <h2>Anomalías de este medidor</h2>
          {anomalies.length === 0 ? (
            <p className="empty">{summaryQuery.data.last_run ? 'No hay anomalías detectadas.' : 'Pendiente de análisis: todavía no hay resultados.'}</p>
          ) : (
            <div className="anomaly-list">
              {anomalies.map((anomaly) => (
                <Link
                  key={anomaly.id}
                  to="/hallazgos/$anomalyId"
                  params={{ anomalyId: String(anomaly.id) }}
                  className={`anomaly sev-${anomaly.severity}`}
                >
                  <span className="anomaly-head">
                    <TypeBadge type={anomaly.type} />
                    <SeverityBadge severity={anomaly.severity} />
                    <StatusBadge status={anomaly.status} />
                    <span className="anomaly-when">
                      {formatDateTime(anomaly.window_start)} →{' '}
                      {formatDateTime(anomaly.window_end)}
                    </span>
                  </span>
                  <p className="anomaly-reason">{anomaly.affected_readings} lecturas afectadas. Abrir el hallazgo para ver su evidencia.</p>
                </Link>
              ))}
            </div>
          )}
        </div>
      </div>

      <div className="flex min-w-0 flex-col gap-4">
        <div className="card">
          <h2>Medidor {meter.meter_id}</h2>
          <p className="muted">Nombre de demostración (dato sintético): {meter.name}. Ubicación: {meter.location || 'sin datos'}.</p>
          <dl className="facts">
            <dt>Medidor</dt>
            <dd>{meter.meter_id}</dd>
            <dt>Salud</dt>
            <dd>
              <HealthBadge health={health} />
            </dd>
            <dt>Consumo del periodo</dt>
            <dd>{formatKWh(total_kwh)}</dd>
            <dt>Lecturas</dt>
            <dd>{points.length}</dd>
            <dt>Muestras de la línea base</dt>
            <dd>{baseline[0]?.samples ?? 0}</dd>
          </dl>
        </div>

        <div className="card">
          <h2>Reportes de contexto</h2>
          {events.length === 0 ? (
            <p className="empty">No hay reportes para este medidor.</p>
          ) : (
            <table>
              <tbody>
                {events.map((event) => (
                  <tr key={`${event.timestamp}-${event.type}`}>
                    <td className="mono">{formatDateTime(event.timestamp)}</td>
                    <td>
                       <span className="badge muted">{event.type === 'UNKNOWN' ? 'Sin evento operativo reportado' : event.type === 'SCHEDULED_OUTAGE' ? 'Parada programada' : event.type === 'OPERATIONAL_CHANGE' ? 'Cambio operativo' : 'Reporte de calidad'}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {anomalies[0] && (
          <div className="card">
            <h2>Mayor desviación detectada</h2>
            <dl className="facts">
              <dt>Consumo real</dt>
              <dd>{formatKWh(anomalies[0].actual_kwh)}</dd>
              <dt>Línea base</dt>
              <dd>{formatKWh(anomalies[0].baseline_kwh)}</dd>
              <dt>Desviación</dt>
               <dd>{formatPercent(anomalies[0].deviation_percent)}</dd>
              <dt>Lecturas afectadas</dt>
              <dd>{anomalies[0].affected_readings}</dd>
            </dl>
          </div>
        )}
      </div>
      </div>

      <div className="card mt-4">
        <h2>Variables eléctricas horarias</h2>
        <p className="muted">El estado de origen no sustituye el veredicto de calidad: las lecturas afectadas se identifican aparte.</p>
        <div className="max-h-96 overflow-auto">
          <table><thead><tr><th>Hora UTC</th><th>Voltaje (V)</th><th>Corriente (A)</th><th>Factor de potencia</th><th>Estado de origen</th><th>Evidencia</th></tr></thead>
            <tbody>{points.map((point) => <tr key={point.timestamp}>
              <td>{formatDateTime(point.timestamp)}</td>
              <td>{point.voltage_v?.toFixed(2)}</td>
              <td>{point.current_a?.toFixed(2)}</td><td>{point.power_factor?.toFixed(3)}</td>
              <td>{point.ingested_status}</td><td>{point.in_anomaly ? 'Lectura afectada' : 'Sin hallazgo'}</td>
            </tr>)}</tbody>
          </table>
        </div>
      </div>
    </div>
  )
}
