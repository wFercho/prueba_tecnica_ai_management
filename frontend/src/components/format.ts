// Readings are UTC. Keep displayed times in that zone so a reported hour never
// shifts when the browser is opened somewhere else.
export function formatNumber(value: number, digits = 0) {
  return value.toLocaleString('es-ES', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

export function formatKWh(value: number) {
  return `${formatNumber(value, value >= 1000 ? 0 : 1)} kWh`
}

export function formatDateTime(iso: string) {
  const at = new Date(iso)
  return `${new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: 'UTC' }).format(at)} UTC`
}

export function formatDay(iso: string) {
  return new Intl.DateTimeFormat('es-ES', { day: '2-digit', month: '2-digit', timeZone: 'UTC' }).format(new Date(iso))
}

export function formatPercent(value: number) {
  const sign = value > 0 ? '+' : ''
  return `${sign}${formatNumber(value, 1)} %`
}
