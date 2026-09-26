import { CartesianGrid, Line, LineChart, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'
import { useState } from 'react'
import { formatDateTime, formatDay, formatKWh, formatPercent } from './format'
import { SortButton } from './SortButton'

export interface ChartPoint {
  at: number
  value: number
  baseline: number | null
  inAnomaly: boolean
}

// A contiguous episode is one band behind the two differently-styled lines.
// Intermittent faulty readings remain individual narrow marks instead of
// shading normal hours between them.
function episodeBands(points: ChartPoint[]) {
  const bands: { first: number; last: number }[] = []
  points.forEach((point, index) => {
    if (!point.inAnomaly) return
    const previous = bands[bands.length - 1]
    if (previous && points[previous.last].at + 60 * 60 * 1000 === point.at) {
      previous.last = index
    } else {
      bands.push({ first: index, last: index })
    }
  })
  return bands
}

export function ConsumptionChart({ points, height = 270, label }: {
  points: ChartPoint[]
  height?: number
  label: string
}) {
  const [hourOrder, setHourOrder] = useState<'asc' | 'desc'>('asc')
  if (points.length === 0) return <p className="empty">No hay lecturas en este periodo.</p>
  const ordered = [...points].sort((a, b) => (hourOrder === 'asc' ? a.at - b.at : b.at - a.at))
  const bands = episodeBands(points)
  const first = points[0]
  const last = points[points.length - 1]
  const ticks = Array.from({ length: Math.ceil((last.at - first.at) / (24 * 3600_000)) + 1 },
    (_, index) => first.at + index * 24 * 3600_000)
  return (
    <>
      <div role="img" aria-label={label} className={height <= 200 ? 'h-52 w-full' : 'h-72 w-full'}>
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={points} margin={{ top: 15, right: 16, left: 0, bottom: 10 }} accessibilityLayer>
            <CartesianGrid vertical={false} stroke="#dce2ed" />
            <XAxis type="number" dataKey="at" domain={['dataMin', 'dataMax']} scale="time"
              ticks={ticks} tickFormatter={(value: number) => formatDay(new Date(value).toISOString())}
              tick={{ fontSize: 11 }} minTickGap={20} />
            <YAxis tick={{ fontSize: 11 }} width={44} unit=" kWh" />
            {bands.map((band) => (
              <ReferenceArea key={points[band.first].at} x1={points[band.first].at - 30 * 60_000}
                x2={points[band.last].at + 30 * 60_000} fill="#d9c5ff" fillOpacity={0.45}
                stroke="none" ifOverflow="extendDomain" />
            ))}
            <Line name="Línea base" dataKey="baseline" stroke="#475569" strokeWidth={2}
              strokeDasharray="5 4" dot={false} activeDot={{ r: 4 }} isAnimationActive={false} connectNulls={false} />
            <Line name="Consumo real" dataKey="value" stroke="#6d28d9" strokeWidth={2.5}
              dot={false} activeDot={{ r: 4 }} isAnimationActive={false} />
            <Tooltip content={({ active, payload }) => {
              if (!active || !payload?.length) return null
              const point = payload[0]?.payload as ChartPoint | undefined
              if (!point) return null
              const deviation = point.baseline && point.baseline > 0
                ? (point.value - point.baseline) / point.baseline * 100 : null
              return <div role="status" className="rounded-md border border-slate-200 bg-white p-3 text-sm shadow-lg">
                <strong>{formatDateTime(new Date(point.at).toISOString())}</strong>
                <p>Consumo real: {formatKWh(point.value)}</p>
                <p>Línea base: {point.baseline == null ? 'Sin datos suficientes' : formatKWh(point.baseline)}</p>
                <p>Desviación: {deviation == null ? 'No disponible' : formatPercent(deviation)}</p>
                {point.inAnomaly && <p>Lectura afectada por el episodio</p>}
              </div>
            }} />
          </LineChart>
        </ResponsiveContainer>
      </div>
      <p className="chart-legend">
        <span><span className="legend-swatch actual" />Consumo real (trazo continuo)</span>
        <span><span className="legend-swatch baseline" />Línea base (trazo discontinuo)</span>
        {bands.length > 0 && <span><span className="legend-swatch window" />Horas del episodio</span>}
        <span className="muted">{formatDateTime(new Date(first.at).toISOString())} → {formatDateTime(new Date(last.at).toISOString())}</span>
      </p>
      <details className="mt-2 text-sm">
        <summary className="cursor-pointer text-blue-700">Consultar lecturas y comparación en tabla</summary>
        <div className="max-h-72 overflow-auto">
          <table><thead><tr><th aria-sort={hourOrder === 'asc' ? 'ascending' : 'descending'}>
            <SortButton
              label="Hora (UTC)"
              sorted={hourOrder}
              onToggle={() => setHourOrder((current) => (current === 'asc' ? 'desc' : 'asc'))}
            />
          </th><th>Real</th><th>Línea base</th><th>Episodio</th></tr></thead>
            <tbody>{ordered.map((point) => <tr key={point.at}>
              <td>{formatDateTime(new Date(point.at).toISOString())}</td>
              <td>{formatKWh(point.value)}</td>
              <td>{point.baseline == null ? 'Sin datos' : formatKWh(point.baseline)}</td>
              <td>{point.inAnomaly ? 'Lectura afectada' : 'No'}</td>
            </tr>)}</tbody>
          </table>
        </div>
      </details>
    </>
  )
}
