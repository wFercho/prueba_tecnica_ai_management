// DateRangeFilter is the shared Desde/Hasta control for the tables that carry
// dates per row. Empty means unbounded on that side; the caller decides the
// matching semantics (overlap for episode windows, containment for hourly rows).
export function DateRangeFilter({
  prefix,
  from,
  to,
  onFromChange,
  onToChange,
  onClear,
}: {
  prefix: string
  from: string
  to: string
  onFromChange: (value: string) => void
  onToChange: (value: string) => void
  onClear: () => void
}) {
  return (
    <div className="mb-4 flex flex-wrap items-end gap-3">
      <div className="flex flex-col gap-1">
        <label htmlFor={`${prefix}-from`}>Desde</label>
        <input
          id={`${prefix}-from`}
          type="date"
          className="h-10 rounded-md border border-slate-300 px-3"
          value={from}
          max={to || undefined}
          onChange={(event) => onFromChange(event.target.value)}
        />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor={`${prefix}-to`}>Hasta</label>
        <input
          id={`${prefix}-to`}
          type="date"
          className="h-10 rounded-md border border-slate-300 px-3"
          value={to}
          min={from || undefined}
          onChange={(event) => onToChange(event.target.value)}
        />
      </div>
      <button type="button" className="action" onClick={onClear} disabled={!from && !to}>
        Limpiar
      </button>
    </div>
  )
}
