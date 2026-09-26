import { ChevronLeft, ChevronRight } from 'lucide-react'

// PaginationControls is the shared Spanish pager for every table: the two
// TanStack tables and the manually sliced plain tables alike.
export function PaginationControls({
  page,
  pages,
  total,
  unit,
  onPrevious,
  onNext,
}: {
  page: number
  pages: number
  total: number
  unit: string
  onPrevious: () => void
  onNext: () => void
}) {
  if (pages < 1) return null
  const current = Math.min(page, pages - 1)
  return (
    <div className="mt-3 flex items-center justify-end gap-2 text-sm">
      <span className="muted" role="status">
        Página {current + 1} de {pages} · {total} {unit}
      </span>
      <button
        type="button"
        className="action"
        onClick={onPrevious}
        disabled={current === 0}
        aria-label="Página anterior"
      >
        <ChevronLeft size={16} aria-hidden="true" /> Anterior
      </button>
      <button
        type="button"
        className="action"
        onClick={onNext}
        disabled={current >= pages - 1}
        aria-label="Página siguiente"
      >
        Siguiente <ChevronRight size={16} aria-hidden="true" />
      </button>
    </div>
  )
}
