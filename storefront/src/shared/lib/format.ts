const currencyFormatter = new Intl.NumberFormat('vi-VN', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
})

const dateFormatter = new Intl.DateTimeFormat('vi-VN', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
})

export function formatPrice(priceCents: number): string {
  return currencyFormatter.format(priceCents / 100)
}

export function formatDate(value: string): string {
  return dateFormatter.format(new Date(value))
}

export function initials(value: string): string {
  return value
    .trim()
    .split(/\s+/)
    .slice(-2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('')
}
