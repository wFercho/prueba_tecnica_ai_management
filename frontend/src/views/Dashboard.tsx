import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Anomaly, type Run } from '../api'
import {
  ConfidenceBadge,
  HealthBadge,
  RunStateBadge,
  SeverityBadge,
  StatusBadge,
  TypeBadge,
} from '../components/badges'
import { formatDateTime, formatKWh, formatNumber } from '../components/format'
import type { Summary } from '../api'

function AnomalyRow({ anomaly }: { anomaly: Anomaly }) {
  return (
    <Link
      to="/hallazgos/$anomalyId"
      params={{ anomalyId: String(anomaly.id) }}
      className={`anomaly sev-${anomaly.severity}`}
    >
      <span className="anomaly-head">
        <span className="anomaly-meter">{anomaly.meter_id}</span>
        <TypeBadge type={anomaly.type} />
        <SeverityBadge severity={anomaly.severity} />
        <ConfidenceBadge band={anomaly.confidence_band} confidence={anomaly.confidence} />
        <StatusBadge status={anomaly.status} />
        <span className="anomaly-when">
          {formatDateTime(anomaly.window_start)} → {formatDateTime(anomaly.window_end)}
        </span>
      </span>
      <p className="anomaly-reason">{anomaly.affected_readings} lecturas afectadas. Abrir el hallazgo para ver su evidencia.</p>
    </Link>
  )
}

export function Dashboard() {
  const queryClient = useQueryClient()
  const summaryQuery = useQuery({ queryKey: ['summary'], queryFn: api.summary })
  const anomaliesQuery = useQuery({ queryKey: ['anomalies'], queryFn: api.anomalies })
  const [runId, setRunId] = useState<number | null>(null)
  const runQuery = useQuery({
    queryKey: ['run', runId],
    queryFn: () => api.analysis(runId!),
    enabled: runId !== null,
    refetchInterval: (query) => (query.state.data?.run.narrating ?? 0) > 0 ? 700 : false,
  })
  const analyze = useMutation({
    mutationFn: api.analyze,
    onSuccess: async ({ run }) => {
      setRunId(run.id)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['summary'] }),
        queryClient.invalidateQueries({ queryKey: ['anomalies'] }),
        queryClient.invalidateQueries({ queryKey: ['meter'] }),
      ])
    },
  })

  if (summaryQuery.isPending || anomaliesQuery.isPending) {
    return <p className="empty" role="status">Cargando el panel general…</p>
  }
  if (summaryQuery.isError || anomaliesQuery.isError) {
    return <p className="notice error" role="alert">No se pudo cargar el panel general. <button type="button" onClick={() => { void summaryQuery.refetch(); void anomaliesQuery.refetch() }}>Reintentar</button></p>
  }

  const summary: Summary = summaryQuery.data
  const anomalies = anomaliesQuery.data.anomalies
  const lastRun: Run | null = runQuery.data?.run ?? summary.last_run
  const narrating = lastRun?.narrating ?? 0
  const working = analyze.isPending || narrating > 0

  return (
    <>
      <section className="grid kpis">
        <div className="card">
          <div className="kpi-value">{anomalies.length}</div>
          <div className="kpi-label">anomalías del último análisis</div>
        </div>
        <div className="card">
          <div className="kpi-value">{summary.needs_attention}</div>
          <div className="kpi-label">pendientes de revisión</div>
        </div>
        <div className="card">
          <div className="kpi-value">{formatNumber(summary.total_kwh)}</div>
          <div className="kpi-label">kWh en el periodo</div>
        </div>
        <div className="card">
          <div className="kpi-value">
            {lastRun ? (
              <RunStateBadge run={lastRun} />
            ) : (
              <span className="muted">Pendiente de análisis</span>
            )}
          </div>
          <div className="kpi-label">
            {lastRun
              ? `${formatDateTime(lastRun.window_start)} → ${formatDateTime(lastRun.window_end)}`
              : 'Ejecuta el análisis para ver los hallazgos'}
          </div>
        </div>
      </section>

      {(analyze.isError || runQuery.isError) && (
        <p className="notice error" role="alert">No se pudo completar el análisis. Inténtalo de nuevo.</p>
      )}

      <div className="grid split">
        <div>
          <div className="card">
            <h2>Hallazgos, por urgencia</h2>
            {anomalies.length === 0 ? (
              <p className="empty">
                {lastRun ? 'No hay anomalías en el último análisis.' : 'Pendiente de análisis: aún no hay resultados.'}
              </p>
            ) : (
              <div className="anomaly-list">
                {anomalies.map((anomaly) => (
                  <AnomalyRow key={anomaly.id} anomaly={anomaly} />
                ))}
              </div>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <h2>Análisis</h2>
            <button
              type="button"
              className="action primary"
              onClick={() => analyze.mutate()}
              disabled={working}
            >
              {working ? (
                <>
                  <span className="spinner" />
                   Analizando…
                </>
              ) : (
                'Ejecutar análisis IA'
              )}
            </button>
            <p className="muted" style={{ marginBottom: 0 }}>
              {narrating > 0
                ? `${narrating} explicación${narrating === 1 ? '' : 'es'} en preparación. Los hallazgos ya están guardados.`
                : 'El detector sigue reglas reproducibles; la narración puede mejorarse más tarde.'}
            </p>
            {lastRun?.state === 'FAILED' && lastRun.error && (
              <p className="notice error" style={{ marginTop: 12, marginBottom: 0 }}>
                El último análisis falló. Inténtalo de nuevo.
              </p>
            )}
          </div>

          <div className="card">
            <h2>Medidores</h2>
            <table>
              <thead>
                <tr>
                  <th>Medidor</th>
                  <th>Salud</th>
                  <th className="num">kWh</th>
                </tr>
              </thead>
              <tbody>
                {summary.meters.map((meter) => (
                  <tr key={meter.meter_id}>
                    <td><Link className="rounded-sm font-medium text-blue-700 underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700" to="/medidores/$meterId" params={{ meterId: meter.meter_id }}>{meter.meter_id}</Link></td>
                    <td>
                      <HealthBadge health={meter.health} />
                    </td>
                    <td className="num">{formatNumber(meter.total_kwh)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {lastRun && (
            <div className="card">
              <h2>Último análisis</h2>
              <dl className="facts">
                <dt>Estado</dt>
                <dd><RunStateBadge run={lastRun} /></dd>
                <dt>Hallazgos</dt>
                <dd>{lastRun.anomaly_count}</dd>
                <dt>Explicados</dt>
                <dd>{lastRun.explained ?? 0}</dd>
                <dt>Fallidos</dt>
                <dd>{lastRun.failed ?? 0}</dd>
                <dt>Consumo total</dt>
                <dd>{formatKWh(summary.total_kwh)}</dd>
              </dl>
            </div>
          )}
        </div>
      </div>
    </>
  )
}
