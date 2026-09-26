// rangeBounds turns YYYY-MM-DD inputs into UTC millisecond bounds for overlap
// and containment checks. An empty side stays unbounded.
export function rangeBounds(from: string, to: string): { start: number; end: number } {
  return {
    start: from ? new Date(`${from}T00:00:00Z`).getTime() : Number.NEGATIVE_INFINITY,
    end: to ? new Date(`${to}T23:59:59.999Z`).getTime() : Number.POSITIVE_INFINITY,
  }
}
