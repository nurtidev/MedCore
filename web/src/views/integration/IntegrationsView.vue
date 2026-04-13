<template>
  <div class="space-y-4">
    <div class="bg-white rounded-xl border p-4">
      <p class="text-sm text-gray-500">
        Управление подключениями к внешним медицинским системам (eGov, DAMUMED, iDoctor, Olymp, Invivo).
      </p>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="bg-white rounded-xl border p-5 animate-pulse">
        <div class="h-5 bg-gray-200 rounded w-1/3 mb-2" />
        <div class="h-4 bg-gray-100 rounded w-1/2" />
      </div>
    </div>

    <div v-else class="space-y-3">
      <div
        v-for="intg in integrations"
        :key="intg.id"
        class="bg-white rounded-xl border p-5"
      >
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div :class="['w-10 h-10 rounded-lg flex items-center justify-center text-lg',
              intg.is_enabled ? 'bg-green-50' : 'bg-gray-100']">
              {{ integrationIcon(intg.integration_type) }}
            </div>
            <div>
              <p class="font-medium text-gray-900">{{ integrationLabel(intg.integration_type) }}</p>
              <p class="text-xs text-gray-500 font-mono">{{ intg.integration_type }}</p>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <BaseBadge :variant="intg.is_enabled ? 'green' : 'gray'">
              {{ intg.is_enabled ? 'Подключено' : 'Отключено' }}
            </BaseBadge>
            <button
              class="text-sm text-primary-600 hover:underline"
              @click="toggleEnabled(intg)"
            >
              {{ intg.is_enabled ? 'Отключить' : 'Включить' }}
            </button>
          </div>
        </div>

        <!-- Config fields (read-only display) -->
        <div v-if="Object.keys(intg.config).length" class="mt-4 grid grid-cols-2 gap-2">
          <div v-for="(val, key) in intg.config" :key="key" class="bg-gray-50 rounded-lg p-2">
            <p class="text-xs text-gray-400 uppercase">{{ key }}</p>
            <p class="text-sm text-gray-700 font-mono truncate">{{ maskSecret(String(val)) }}</p>
          </div>
        </div>
      </div>

      <div v-if="!integrations.length" class="bg-white rounded-xl border p-8 text-center text-sm text-gray-400">
        Нет настроенных интеграций
      </div>
    </div>

    <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { integrationApi } from '@/api/integration'
import type { IntegrationConfig } from '@/api/integration'
import BaseBadge from '@/components/ui/BaseBadge.vue'

const loading = ref(false)
const integrations = ref<IntegrationConfig[]>([])
const error = ref('')

const LABELS: Record<string, string> = {
  egov: 'eGov (госуслуги)',
  damumed: 'DAMUMED',
  idoctor: 'iDoctor',
  olymp: 'Olymp Lab',
  invivo: 'Invivo Lab',
}

const ICONS: Record<string, string> = {
  egov: '🏛️',
  damumed: '🏥',
  idoctor: '👨‍⚕️',
  olymp: '🔬',
  invivo: '🧪',
}

function integrationLabel(type: string) {
  return LABELS[type] ?? type
}

function integrationIcon(type: string) {
  return ICONS[type] ?? '🔌'
}

function maskSecret(val: string) {
  if (val.length <= 6) return '••••••'
  return val.slice(0, 3) + '•'.repeat(val.length - 6) + val.slice(-3)
}

async function toggleEnabled(intg: IntegrationConfig) {
  error.value = ''
  try {
    const { data } = await integrationApi.updateIntegration(intg.id, {
      is_enabled: !intg.is_enabled,
    })
    const idx = integrations.value.findIndex(i => i.id === intg.id)
    if (idx !== -1) integrations.value[idx] = data
  } catch {
    error.value = 'Не удалось обновить интеграцию. Попробуйте позже.'
  }
}

onMounted(async () => {
  loading.value = true
  try {
    const { data } = await integrationApi.listIntegrations()
    integrations.value = data
  } finally {
    loading.value = false
  }
})
</script>
