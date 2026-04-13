import { ref, computed } from 'vue'

export function usePagination(pageSize = 20) {
  const page = ref(1)
  const total = ref(0)

  const offset = computed(() => (page.value - 1) * pageSize)
  const totalPages = computed(() => Math.ceil(total.value / pageSize))
  const hasPrev = computed(() => page.value > 1)
  const hasNext = computed(() => page.value < totalPages.value)

  function prev() { if (hasPrev.value) page.value-- }
  function next() { if (hasNext.value) page.value++ }
  function goTo(p: number) { page.value = Math.max(1, Math.min(p, totalPages.value)) }
  function reset() { page.value = 1 }

  return { page, total, offset, totalPages, hasPrev, hasNext, prev, next, goTo, reset, pageSize }
}
