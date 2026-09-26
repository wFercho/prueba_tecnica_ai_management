import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type ColumnDef, type SortingState } from '@tanstack/react-table'
import type { Anomaly } from '../api'
import { ConfidenceBadge, SeverityBadge, TypeBadge } from '../components/badges'
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
  // The API orders by operational urgency, not by raw confidence across types.
  // eslint-disable-next-line react-hooks/incompatible-library
  const table = useReactTable({ data: anomalies, columns, state: { sorting }, onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel() })
  return <div className="overflow-x-auto"><table className="min-w-full">
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
}
