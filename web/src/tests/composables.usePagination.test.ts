import { describe, it, expect } from 'vitest'
import { usePagination } from '@/composables/usePagination'

describe('usePagination', () => {
  it('initialises at page 1', () => {
    const p = usePagination(10)
    expect(p.page.value).toBe(1)
    expect(p.offset.value).toBe(0)
  })

  it('computes totalPages correctly', () => {
    const p = usePagination(10)
    p.total.value = 35
    expect(p.totalPages.value).toBe(4)
  })

  it('goTo advances page and updates offset', () => {
    const p = usePagination(10)
    p.total.value = 50
    p.goTo(3)
    expect(p.page.value).toBe(3)
    expect(p.offset.value).toBe(20)
  })

  it('hasPrev / hasNext are correct', () => {
    const p = usePagination(10)
    p.total.value = 30
    p.goTo(2)
    expect(p.hasPrev.value).toBe(true)
    expect(p.hasNext.value).toBe(true)
    p.goTo(1)
    expect(p.hasPrev.value).toBe(false)
    p.goTo(3)
    expect(p.hasNext.value).toBe(false)
  })

  it('does not go below page 1', () => {
    const p = usePagination(10)
    p.total.value = 20
    p.prev()
    expect(p.page.value).toBe(1)
  })

  it('does not exceed totalPages', () => {
    const p = usePagination(10)
    p.total.value = 20
    p.next()
    p.next()
    p.next()
    expect(p.page.value).toBe(2)
  })
})
