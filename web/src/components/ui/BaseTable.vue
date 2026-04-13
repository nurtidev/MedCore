<template>
  <div>
    <div class="overflow-x-auto rounded-lg border border-gray-200">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
            >
              {{ col.label }}
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-if="loading">
            <td :colspan="columns.length" class="px-4 py-8 text-center">
              <div class="flex justify-center">
                <svg class="animate-spin h-6 w-6 text-primary-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
                </svg>
              </div>
            </td>
          </tr>
          <tr v-else-if="!rows.length">
            <td :colspan="columns.length" class="px-4 py-8 text-center text-sm text-gray-400">
              {{ $t('common.no_data') }}
            </td>
          </tr>
          <tr v-for="(row, i) in rows" :key="i" class="hover:bg-gray-50 transition-colors">
            <slot :row="row" :index="i" />
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-between px-4 py-3 mt-2">
      <p class="text-sm text-gray-600">
        {{ $t('common.total') }}: {{ total }}
      </p>
      <div class="flex gap-1">
        <button
          :disabled="page <= 1"
          class="px-3 py-1 text-sm rounded border disabled:opacity-40 hover:bg-gray-50"
          @click="$emit('page-change', page - 1)"
        >←</button>
        <span class="px-3 py-1 text-sm">{{ page }} / {{ totalPages }}</span>
        <button
          :disabled="page >= totalPages"
          class="px-3 py-1 text-sm rounded border disabled:opacity-40 hover:bg-gray-50"
          @click="$emit('page-change', page + 1)"
        >→</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  columns: { key: string; label: string }[]
  rows: unknown[]
  loading?: boolean
  total?: number
  page?: number
  totalPages?: number
}>(), { page: 1, totalPages: 1 })
defineEmits<{ 'page-change': [page: number] }>()
</script>
