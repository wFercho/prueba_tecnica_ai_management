import { useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { flexRender, getCoreRowModel, getPaginationRowModel, getSortedRowModel, useReactTable, type ColumnDef, type PaginationState, type SortingState } from '@tanstack/react-table'
import type { Anomaly } from '../api'
import { ConfidenceBadge, SeverityBadge, TypeBadge } from '../components/badges'
import { DateRangeFilter } from '../components/DateRangeFilter'
import { rangeBounds } from '../components/dateRange'
import { PaginationControls } from '../components/PaginationControls'
import { SortButton } from '../components/SortButton'

const columns: ColumnDef<Anomaly>[] = [
  { accessorKey: 'meter_id', header: 'Medidor',
    cell: ({ row }) => <span className="font-medium">{row.original.meter_id}</span> },
  { accessorKey: 'type', header: 'Tipo', cell: ({ row }) => <TypeBadge type={row.original.type} /> },
  { accessorKey: 'severity', header: 'Severidad', cell: ({ row }) => <SeverityBadge severity={row.original.severity} /> },
  { accessorKey: 'confidence', header: 'Confianza', cell: ({ row }) =>
    <ConfidenceBadge band={row.original.confidence_band} confidence={row.original.confidence} /> },
  { accessorKey: 'recommended_action', header: 'Acción recomendada', enableSorting: false,
    cell: ({ row }) => <span>{row.original.recommended_action}</span> },
  { id: 'detail', header: 'Detalle', enableSorting: false,
    cell: ({ row }) => (
      <Link to="/hallazgos/$anomalyId" params={{ anomalyId: String(row.original.id) }}
        className="inline-flex items-center gap-1 font-medium text-blue-700 underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
        aria-label={`Ver hallazgo ${row.original.id} del medidor ${row.original.meter_id}`}>
        Ver <ArrowRight size={16} aria-hidden="true" />
      </Link>
    ) },
]

export function AnomalyTable({ anomalies }: { anomalies: Anomaly[] }) {
  const [sorting, setSorting] = useState<SortingState>([])
  const [pagination, setPagination] = useState<PaginationState>({ pageIndex: 0, pageSize: 10 })
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  // An episode overlaps the range when it ends at or after the start and
  // starts at or before the end, so a long episode is never hidden for
  // starting before Desde. Memoized so the page only resets when the rows
  // actually change, not on every render.
  const visible = useMemo(() => {
    const { start, end } = rangeBounds(from, to)
    return anomalies.filter((anomaly) => {
      const episodeStart = new Date(anomaly.window_start).getTime()
      const episodeEnd = new Date(anomaly.window_end).getTime()
      return episodeStart <= end && episodeEnd >= start
    })
  }, [anomalies, from, to])
  // The API orders by operational urgency, not by raw confidence across types.
  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({ data: visible, columns,
    state: { sorting, pagination },
    onSortingChange: (update) => {
      setSorting(update)
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
    onPaginationChange: setPagination,
    getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })
  return <>
    <DateRangeFilter prefix="hallazgos" from={from} to={to}
      onFromChange={(value) => { setFrom(value); setPagination((current) => ({ ...current, pageIndex: 0 })) }}
      onToChange={(value) => { setTo(value); setPagination((current) => ({ ...current, pageIndex: 0 })) }}
      onClear={() => { setFrom(''); setTo(''); setPagination((current) => ({ ...current, pageIndex: 0 })) }} />
    {(from || to) && (
      <p className="muted" role="status">{visible.length} de {anomalies.length} hallazgos en el rango.</p>
    )}
    {visible.length === 0 ? (
      <p className="empty">No hay hallazgos en el rango elegido.</p>
    ) : (
    <div className="overflow-x-auto"><table className="min-w-full">
    <thead>{table.getHeaderGroups().map((group) => <tr key={group.id}>{group.headers.map((header) =>
      <th key={header.id} aria-sort={header.column.getIsSorted() === 'asc' ? 'ascending' : header.column.getIsSorted() === 'desc' ? 'descending' : 'none'}>
        {header.column.getCanSort() ? (
          <SortButton
            label={String(header.column.columnDef.header)}
            sorted={header.column.getIsSorted()}
            onToggle={(event) => header.column.getToggleSortingHandler()?.(event)}
          />
        ) : (
          flexRender(header.column.columnDef.header, header.getContext())
        )}
      </th>)}</tr>)}</thead>
    <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id}>
      {row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}
    </tr>)}</tbody>
  </table></div>
    )}
  <PaginationControls
    page={table.getState().pagination.pageIndex}
    pages={table.getPageCount()}
    total={visible.length}
    unit="hallazgos"
    onPrevious={() => table.previousPage()}
    onNext={() => table.nextPage()}
  />
  </>
}
