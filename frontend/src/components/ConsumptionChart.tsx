import { formatDateTime, formatDay, formatKWh } from './format'

export interface ChartPoint {
  at: number
  value: number
  baseline: number | null
  inAnomaly: boolean
}

// A hand-drawn SVG chart rather than a charting library: the whole chart is one
// polyline, one shaded band and two axes, and adding a dependency for that would
// cost more than it saves. It scales with the data, so the same component shows a
// flat meter and a 110% spike without special cases.
export function ConsumptionChart({
  points,
  height = 240,
  label,
}: {
  points: ChartPoint[]
  height?: number
  label: string
}) {
  if (points.length === 0) {
    return <p className="empty">No hay lecturas en este periodo.</p>
  }

  const width = 960
  const padLeft = 52
  const padRight = 16
  const padTop = 12
  const padBottom = 26
  const plotWidth = width - padLeft - padRight
  const plotHeight = height - padTop - padBottom

  const values = points.flatMap((point) =>
    [point.value, point.baseline].filter((v): v is number => v !== null),
  )
  const max = Math.max(...values, 1)
  const min = Math.min(...values, 0)
  // Headroom so the peak is not drawn on the frame.
  const top = max + (max - min) * 0.08
  const scaleY = (value: number) => padTop + plotHeight - (value / top) * plotHeight
  const scaleX = (index: number) =>
    padLeft + (index / Math.max(points.length - 1, 1)) * plotWidth

  const line = (pick: (point: ChartPoint) => number | null) =>
    points
      .map((point, index) => {
        const value = pick(point)
        if (value === null) return null
        return `${index === 0 ? 'M' : 'L'}${scaleX(index).toFixed(1)},${scaleY(value).toFixed(1)}`
      })
      .filter(Boolean)
      .join(' ')

  // One rectangle per contiguous run of anomalous hours, so an episode reads as a
  // band rather than as 58 separate marks.
  const bands: { from: number; to: number }[] = []
  points.forEach((point, index) => {
    const last = bands[bands.length - 1]
    if (!point.inAnomaly) return
    if (last && last.to === index - 1) last.to = index
    else bands.push({ from: index, to: index })
  })

  const ticks = [0, 0.5, 1].map((fraction) => ({
    value: top * fraction,
    y: scaleY(top * fraction),
  }))

  const first = points[0]
  const last = points[points.length - 1]
  const windowBand = bands.find((band) => band.to - band.from > 2)

  return (
    <>
      <svg
        className="chart"
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={label}
        preserveAspectRatio="none"
      >
        {bands.map((band) => (
          <rect
            key={`${band.from}-${band.to}`}
            x={scaleX(band.from)}
            y={padTop}
            width={Math.max(scaleX(band.to) - scaleX(band.from), 2)}
            height={plotHeight}
            fill="color-mix(in srgb, var(--high) 10%, transparent)"
          />
        ))}

        {ticks.map((tick) => (
          <g key={tick.value}>
            <line
              x1={padLeft}
              x2={width - padRight}
              y1={tick.y}
              y2={tick.y}
              stroke="var(--line)"
              strokeWidth={1}
            />
            <text
              x={padLeft - 8}
              y={tick.y + 3}
              textAnchor="end"
              fontSize={10}
              fill="var(--ink-faint)"
            >
              {tick.value.toFixed(0)}
            </text>
          </g>
        ))}

        <path d={line((point) => point.baseline)} fill="none" stroke="var(--baseline)" strokeWidth={1.5} />
        <path d={line((point) => point.value)} fill="none" stroke="var(--actual)" strokeWidth={1.75} />

        <text x={padLeft} y={height - 8} fontSize={10} fill="var(--ink-faint)">
          {formatDay(new Date(first.at).toISOString())}
        </text>
        <text x={width - padRight} y={height - 8} textAnchor="end" fontSize={10} fill="var(--ink-faint)">
          {formatDay(new Date(last.at).toISOString())}
        </text>
        {windowBand && (
          <text
            x={scaleX(windowBand.from) + 4}
            y={padTop + 10}
            fontSize={10}
            fill="var(--high)"
          >
            episodio
          </text>
        )}
      </svg>
      <p className="chart-legend">
        <span>
          <span className="legend-swatch actual" />
          consumo real
        </span>
        <span>
          <span className="legend-swatch baseline" />
          baseline
        </span>
        <span>
          <span className="legend-swatch window" />
          horas del episodio
        </span>
        <span className="muted">
          {formatDateTime(new Date(first.at).toISOString())} →{' '}
          {formatDateTime(new Date(last.at).toISOString())} · máximo {formatKWh(max)}
        </span>
      </p>
    </>
  )
}
