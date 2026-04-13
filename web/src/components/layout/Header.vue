<template>
  <header class="bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between">
    <h1 class="text-xl font-semibold text-gray-900">{{ title }}</h1>
    <div class="flex items-center gap-3">
      <!-- Language switcher -->
      <button
        class="text-sm text-gray-500 hover:text-gray-700 px-2 py-1 rounded hover:bg-gray-100 transition"
        @click="toggleLocale"
      >
        {{ locale === 'ru' ? 'ҚАЗ' : 'РУС' }}
      </button>
    </div>
  </header>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { computed } from 'vue'

const { locale, t } = useI18n()
const route = useRoute()

const title = computed(() => {
  const key = `nav.${String(route.name ?? 'dashboard').replace('-', '_')}`
  return t(key, route.meta.title as string ?? 'MedCore')
})

function toggleLocale() {
  locale.value = locale.value === 'ru' ? 'kk' : 'ru'
  localStorage.setItem('locale', locale.value)
}
</script>
