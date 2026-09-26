import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import type { MouseEvent } from 'react'

// SortButton is the shared sortable-header control for the data tables. It
// inherits the surrounding `th` typography so it reads as a column header with
// a state icon, not as a body button; the `th` itself keeps `aria-sort`.
export function SortButton({
  label,
  sorted,
  onToggle,
}: {
  label: string
  sorted: false | 'asc' | 'desc'
  onToggle: (event: MouseEvent<HTMLButtonElement>) => void
}) {
  const Icon = sorted === 'asc' ? ArrowUp : sorted === 'desc' ? ArrowDown : ArrowUpDown
  return (
    <button type="button" className="th-sort" onClick={onToggle} aria-label={`Ordenar por ${label}`}>
      {label}
      <Icon size={14} aria-hidden="true" className={sorted ? undefined : 'opacity-40'} />
    </button>
  )
}
