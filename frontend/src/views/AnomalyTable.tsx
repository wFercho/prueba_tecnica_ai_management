import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type ColumnDef, type SortingState } from '@tanstack/react-table'
import type { Anomaly } from '../api'
import { ConfidenceBadge, SeverityBadge, TypeBadge } from '../components/badges'

const columns: ColumnDef<Anomaly>[] = [
  { accessorKey: 'meter_id', header: 'Medidor', cell: ({ row }) =>
    <Link className="font-semibold text-blue-700 underline" to="/hallazgos/$anomalyId" params={{ anomalyId: String(row.original.id) }}>
      {row.original.meter_id}</Link> },
  { accessorKey: 'type', header: 'Tipo', cell: ({ row }) => <TypeBadge type={row.original.type} /> },
  { accessorKey: 'severity', header: 'Severidad', cell: ({ row }) => <SeverityBadge severity={row.original.severity} /> },
  { accessorKey: 'confidence', header: 'Confianza', cell: ({ row }) =>
    <ConfidenceBadge band={row.original.confidence_band} confidence={row.original.confidence} /> },
  { accessorKey: 'recommended_action', header: 'Acción recomendada', enableSorting: false,
    cell: ({ row }) => <span>{row.original.recommended_action}</span> },
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
        {header.column.getCanSort() ? <button type="button" className="cursor-pointer text-left hover:underline"
          onClick={header.column.getToggleSortingHandler()} aria-label={`Ordenar por ${String(header.column.columnDef.header)}`}>
          {flexRender(header.column.columnDef.header, header.getContext())}
          {header.column.getIsSorted() === 'asc' ? ' ↑' : header.column.getIsSorted() === 'desc' ? ' ↓' : ''}
        </button> : flexRender(header.column.columnDef.header, header.getContext())}
      </th>)}</tr>)}</thead>
    <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id}>
      {row.getVisibleCells().map((cell) => <td key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}
    </tr>)}</tbody>
  </table></div>
}
