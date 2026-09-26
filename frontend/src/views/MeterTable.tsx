import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ChevronDown, Search } from 'lucide-react'
import {
  flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable,
  type ColumnDef, type ColumnFiltersState, type PaginationState, type SortingState,
} from '@tanstack/react-table'
import type { Health, Meter, Severity } from '../api'
import { HealthBadge, SeverityBadge } from '../components/badges'
import { SortButton } from '../components/SortButton'
import { PaginationControls } from '../components/PaginationControls'
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
  const [pagination, setPagination] = useState<PaginationState>({ pageIndex: 0, pageSize: 10 })
  // TanStack Table owns mutable table functions; React Compiler must not memoize them.
  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({
    data: meters, columns, state: { sorting, columnFilters: filters, pagination },
    onSortingChange: (update) => {
      setSorting(update)
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
    onColumnFiltersChange: (update) => {
      setFilters(update)
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(), getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(), getPaginationRowModel: getPaginationRowModel(),
  })
  const health = (table.getColumn('health')?.getFilterValue() as string | undefined) ?? ''
  return <>
    <div className="mb-4 flex flex-wrap items-end gap-3">
      <div className="flex flex-col gap-1">
        <label htmlFor="meter-search">Buscar por identificador</label>
        <div className="relative">
          <Search size={16} aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-slate-400" />
          <input id="meter-search" className="h-10 rounded-md border border-slate-300 py-2 pr-3 pl-9" type="search"
            value={(table.getColumn('meter_id')?.getFilterValue() as string) ?? ''}
            onChange={(event) => table.getColumn('meter_id')?.setFilterValue(event.target.value)} />
        </div>
      </div>
      <div className="flex flex-col gap-1">
        <label htmlFor="meter-health">Filtrar por salud</label>
        <div className="relative">
          <select id="meter-health" className="h-10 appearance-none rounded-md border border-slate-300 py-2 pr-9 pl-3" value={health}
            onChange={(event) => table.getColumn('health')?.setFilterValue(event.target.value || undefined)}>
            <option value="">Todos</option><option value="HEALTHY">Normales</option>
            <option value="ALERT">Alertas</option><option value="CRITICAL">Críticos</option>
          </select>
          <ChevronDown size={16} aria-hidden="true" className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-slate-400" />
        </div>
      </div>
    </div>
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>{table.getHeaderGroups().map((group) => <tr key={group.id}>
          {group.headers.map((header) => <th key={header.id}
            aria-sort={header.column.getIsSorted() === 'asc' ? 'ascending' : header.column.getIsSorted() === 'desc' ? 'descending' : 'none'}>
            {header.column.getCanSort() ? (
              <SortButton
                label={String(header.column.columnDef.header)}
                sorted={header.column.getIsSorted()}
                onToggle={(event) => header.column.getToggleSortingHandler()?.(event)}
              />
            ) : (
              flexRender(header.column.columnDef.header, header.getContext())
            )}
          </th>)}
        </tr>)}</thead>
        <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id}>
          {row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}
        </tr>)}</tbody>
      </table>
      {table.getRowModel().rows.length === 0 && <p className="empty">No hay medidores que coincidan con la búsqueda o el filtro.</p>}
      <PaginationControls
        page={table.getState().pagination.pageIndex}
        pages={table.getPageCount()}
        total={table.getFilteredRowModel().rows.length}
        unit="medidores"
        onPrevious={() => table.previousPage()}
        onNext={() => table.nextPage()}
      />
    </div>
  </>
}
