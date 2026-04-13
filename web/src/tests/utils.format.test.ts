import { describe, it, expect } from 'vitest'
import { formatKZT, formatDate, formatPercent } from '@/utils/format'

describe('formatKZT', () => {
  it('formats integer amounts', () => {
    const result = formatKZT(49900)
    expect(result).toContain('49')
    expect(result).toContain('900')
    expect(result).toContain('₸')
  })

  it('formats zero', () => {
    expect(formatKZT(0)).toContain('0')
  })
})

describe('formatPercent', () => {
  it('formats integer percent value', () => {
    expect(formatPercent(75)).toBe('75%')
  })

  it('rounds fractional values', () => {
    expect(formatPercent(75.6)).toBe('76%')
  })

  it('formats zero', () => {
    expect(formatPercent(0)).toBe('0%')
  })

  it('formats 100', () => {
    expect(formatPercent(100)).toBe('100%')
  })
})

describe('formatDate', () => {
  it('returns em dash for empty string', () => {
    expect(formatDate('')).toBe('—')
  })

  it('returns em dash for null/undefined', () => {
    expect(formatDate(null as any)).toBe('—')
  })

  it('returns formatted date string for valid ISO', () => {
    const result = formatDate('2025-05-01T00:00:00Z')
    expect(typeof result).toBe('string')
    expect(result).not.toBe('—')
  })
})
