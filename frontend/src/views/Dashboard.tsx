import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Run } from '../api'
import {
  BandOnly,
  RunStateBadge,
} from '../components/badges'
import { formatDateTime, formatKWh, formatNumber } from '../components/format'
import type { Summary } from '../api'
import { MeterTable } from './MeterTable'
import { AnomalyTable } from './AnomalyTable'

export function Dashboard() {
  const queryClient = useQueryClient()
  const [runId, setRunId] = useState<number | null>(null)
  const summaryQuery = useQuery({ queryKey: ['summary'], queryFn: api.summary })
  const activeRunId = runId ?? ((summaryQuery.data?.last_run?.narrating ?? 0) > 0 ? summaryQuery.data?.last_run?.id ?? null : null)
  const runQuery = useQuery({
    queryKey: ['run', activeRunId],
    queryFn: () => api.analysis(activeRunId!),
    enabled: activeRunId !== null,
    refetchInterval: (query) => (query.state.data?.run.narrating ?? 0) > 0 ? 700 : false,
  })
  const anomaliesQuery = useQuery({ queryKey: ['anomalies'], queryFn: api.anomalies })
  const pendingNarration = runQuery.data?.run.narrating ?? 0
  const previousNarration = useRef<number | null>(null)
  useEffect(() => {
    if (activeRunId === null || !runQuery.data) return
    if (previousNarration.current === pendingNarration) return
    previousNarration.current = pendingNarration
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: ['summary'] }),
      queryClient.invalidateQueries({ queryKey: ['anomalies'] }),
      queryClient.invalidateQueries({ queryKey: ['anomaly'] }),
      queryClient.invalidateQueries({ queryKey: ['meter'] }),
    ])
  }, [pendingNarration, activeRunId, runQuery.data, queryClient])
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
  const lastSuccessful = summary.last_successful_run
  const narrating = lastRun?.narrating ?? 0
  const working = analyze.isPending || narrating > 0

  return (
    <>
      <section className="grid kpis">
        <div className="card">
          <div className="kpi-value">{lastSuccessful ? anomalies.length : 'Pendiente'}</div>
          <div className="kpi-label">anomalías del último análisis</div>
        </div>
        <div className="card">
          <div className="kpi-value">{lastSuccessful ? summary.high_priority : 'Pendiente'}</div>
          <div className="kpi-label">prioridad alta · severidad HIGH</div>
        </div>
        <div className="card">
          <div className="kpi-value">{formatNumber(summary.total_kwh)}</div>
          <div className="kpi-label">kWh en el periodo</div>
        </div>
        <div className="card">
          <div className="kpi-value">{summary.meters.length}</div>
          <div className="kpi-label">medidores</div>
        </div>
        <div className="card">
          <div className="kpi-value">{summary.priority_confidence && anomalies[0]
            ? <><BandOnly band={summary.priority_confidence} /> <span className="text-base">({anomalies[0].meter_id})</span></>
            : 'Pendiente'}</div>
          <div className="kpi-label">confianza del caso más urgente (evidencia, no probabilidad)</div>
          {lastSuccessful && <p className="muted">Bandas: alta {summary.confidence_bands.HIGH ?? 0} · media {summary.confidence_bands.MEDIUM ?? 0} · baja {summary.confidence_bands.LOW ?? 0}</p>}
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
              ? `Intento: ${formatDateTime(lastRun.started_at)}`
              : 'Ejecuta el análisis para ver los hallazgos'}
          </div>
        </div>
      </section>

      {lastRun?.state === 'FAILED' && lastSuccessful && (
        <p role="status" className="notice error">Resultados anteriores: el último análisis falló. Los hallazgos mostrados corresponden al run {lastSuccessful.id}.</p>
      )}

      {(analyze.isError || runQuery.isError) && (
        <p className="notice error" role="alert">No se pudo completar el análisis. Inténtalo de nuevo.</p>
      )}

      <div className="grid split">
        <div>
          <div className="card">
            <h2>Hallazgos, por urgencia</h2>
            {anomalies.length === 0 ? (
              <p className="empty">
                {lastSuccessful ? 'No hay anomalías en el último análisis exitoso.' : 'Pendiente de análisis: aún no hay resultados.'}
              </p>
            ) : (
              <AnomalyTable anomalies={anomalies} />
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
            <MeterTable meters={summary.meters} />
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
