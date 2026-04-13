export function formatKZT(amount: number | string): string {
  return new Intl.NumberFormat('ru-KZ', {
    style: 'currency',
    currency: 'KZT',
    minimumFractionDigits: 0,
  }).format(Number(amount))
}

export function formatDate(date: string): string {
  if (!date) return '—'
  return new Intl.DateTimeFormat('ru-KZ', {
    day: '2-digit',
    month: 'long',
    year: 'numeric',
  }).format(new Date(date))
}

export function formatDateTime(date: string): string {
  if (!date) return '—'
  return new Intl.DateTimeFormat('ru-KZ', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(date))
}

export function formatPercent(value: number): string {
  return `${Math.round(value)}%`
}
