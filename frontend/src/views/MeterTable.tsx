import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import {
  flexRender, getCoreRowModel, getFilteredRowModel, getSortedRowModel, useReactTable,
  type ColumnDef, type ColumnFiltersState, type SortingState,
} from '@tanstack/react-table'
import type { Health, Meter, Severity } from '../api'
import { HealthBadge, SeverityBadge } from '../components/badges'
import { formatNumber, formatPercent } from '../components/format'

const severityRank: Record<string, number> = { HIGH: 0, MEDIUM: 1, LOW: 2 }
const healthRank: Record<Health, number> = {
  CRITICAL: 0, ALERT: 1, HEALTHY: 2, INSUFFICIENT_DATA: 3, UNASSESSED: 4,
}

const columns: ColumnDef<Meter>[] = [
  { accessorKey: 'meter_id', header: 'Medidor', cell: ({ row }) =>
    <Link to="/medidores/$meterId" params={{ meterId: row.original.meter_id }} className="text-blue-700 underline underline-offset-2">
      {row.original.meter_id}</Link> },
  { accessorKey: 'name', header: 'Nombre / ubicación (sintéticos)', enableSorting: false, cell: ({ row }) =>
    <span>{row.original.name} · {row.original.location || 'Sin ubicación'}</span> },
  { accessorKey: 'total_kwh', header: 'Consumo', cell: ({ row }) => `${formatNumber(row.original.total_kwh)} kWh` },
  { accessorKey: 'variation_percent', header: 'Variación',
    sortingFn: (a, b) => {
      const left = a.original.variation_percent
      const right = b.original.variation_percent
      if (left == null && right == null) return 0
      if (left == null) return 1
      if (right == null) return -1
      return left - right
    },
    cell: ({ row }) =>
    row.original.variation_percent == null ? 'Sin datos suficientes' : formatPercent(row.original.variation_percent) },
  { accessorKey: 'health', header: 'Salud',
    sortingFn: (a, b) => healthRank[a.original.health] - healthRank[b.original.health],
    cell: ({ row }) => <HealthBadge health={row.original.health} /> },
  { accessorKey: 'worst_severity', header: 'Anomalía',
    sortingFn: (a, b) => {
      const left: Severity | undefined = a.original.worst_severity
      const right: Severity | undefined = b.original.worst_severity
      return (left ? severityRank[left] : 3) - (right ? severityRank[right] : 3)
    },
    cell: ({ row }) => row.original.worst_severity
    ? <SeverityBadge severity={row.original.worst_severity} />
    : row.original.health === 'UNASSESSED' ? 'Pendiente de análisis' : 'Sin anomalía detectada' },
]

export function MeterTable({ meters }: { meters: Meter[] }) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [filters, setFilters] = useState<ColumnFiltersState>([])
  // TanStack Table owns mutable table functions; React Compiler must not memoize them.
  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({
    data: meters, columns, state: { sorting, columnFilters: filters },
    onSortingChange: setSorting, onColumnFiltersChange: setFilters,
    getCoreRowModel: getCoreRowModel(), getFilteredRowModel: getFilteredRowModel(), getSortedRowModel: getSortedRowModel(),
  })
  const health = (table.getColumn('health')?.getFilterValue() as string | undefined) ?? ''
  return <>
    <div className="mb-4 flex flex-wrap items-end gap-3">
      <div className="flex flex-col gap-1">
        <label htmlFor="meter-search">Buscar por identificador</label>
        <input id="meter-search" className="rounded-md border border-slate-300 p-2" type="search"
          value={(table.getColumn('meter_id')?.getFilterValue() as string) ?? ''}
          onChange={(event) => table.getColumn('meter_id')?.setFilterValue(event.target.value)} />
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="meter-health">Filtrar por salud</label>
        <select id="meter-health" className="rounded-md border border-slate-300 p-2" value={health}
          onChange={(event) => table.getColumn('health')?.setFilterValue(event.target.value || undefined)}>
          <option value="">Todos</option><option value="HEALTHY">Normales</option>
          <option value="ALERT">Alertas</option><option value="CRITICAL">Críticos</option>
        </select>
      </div>
    </div>
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>{table.getHeaderGroups().map((group) => <tr key={group.id}>
          {group.headers.map((header) => <th key={header.id}
            aria-sort={header.column.getIsSorted() === 'asc' ? 'ascending' : header.column.getIsSorted() === 'desc' ? 'descending' : 'none'}>
            {header.column.getCanSort() ? <button type="button" className="cursor-pointer text-left underline-offset-2 hover:underline"
              onClick={header.column.getToggleSortingHandler()} aria-label={`Ordenar por ${String(header.column.columnDef.header)}`}>
              {flexRender(header.column.columnDef.header, header.getContext())}
              {header.column.getIsSorted() === 'asc' ? ' ↑' : header.column.getIsSorted() === 'desc' ? ' ↓' : ''}
            </button> : flexRender(header.column.columnDef.header, header.getContext())}
          </th>)}
        </tr>)}</thead>
        <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id}>
          {row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}
        </tr>)}</tbody>
      </table>
      {table.getRowModel().rows.length === 0 && <p className="empty">No hay medidores que coincidan con la búsqueda o el filtro.</p>}
    </div>
  </>
}
