<template>
  <div class="space-y-4">
    <!-- Toolbar -->
    <div class="bg-white rounded-xl border p-4 flex flex-wrap gap-3 items-center">
      <BaseInput
        v-model="search"
        placeholder="Поиск по имени или email…"
        class="w-64"
        @input="debouncedLoad"
      />
      <select
        v-model="roleFilter"
        class="border border-gray-300 rounded-lg px-3 py-2 text-sm"
        @change="load"
      >
        <option value="">Все роли</option>
        <option v-for="r in roles" :key="r.value" :value="r.value">{{ r.label }}</option>
      </select>
      <RouterLink to="/users/new" class="ml-auto">
        <BaseButton>+ Добавить врача</BaseButton>
      </RouterLink>
    </div>

    <!-- Table -->
    <div class="bg-white rounded-xl border overflow-hidden">
      <div v-if="loading" class="p-8 flex justify-center"><LoadingSpinner /></div>
      <template v-else>
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th v-for="col in cols" :key="col"
                class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                {{ col }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            <tr v-if="!users.length">
              <td colspan="6" class="px-4 py-8 text-center text-sm text-gray-400">
                {{ $t('common.no_data') }}
              </td>
            </tr>
            <tr v-for="u in users" :key="u.id" class="hover:bg-gray-50">
              <td class="px-4 py-3">
                <div class="flex items-center gap-3">
                  <div class="w-8 h-8 rounded-full bg-primary-100 text-primary-700 flex items-center justify-center text-sm font-semibold">
                    {{ u.first_name[0] }}{{ u.last_name[0] }}
                  </div>
                  <div>
                    <p class="text-sm font-medium text-gray-900">{{ u.first_name }} {{ u.last_name }}</p>
                    <p class="text-xs text-gray-500">{{ u.email }}</p>
                  </div>
                </div>
              </td>
              <td class="px-4 py-3 text-sm text-gray-600">{{ u.phone || '—' }}</td>
              <td class="px-4 py-3">
                <BaseBadge :variant="roleBadge(u.role)">{{ $t(`roles.${u.role}`) }}</BaseBadge>
              </td>
              <td class="px-4 py-3">
                <BaseBadge :variant="u.is_active ? 'green' : 'gray'">
                  {{ u.is_active ? 'Активен' : 'Неактивен' }}
                </BaseBadge>
              </td>
              <td class="px-4 py-3 text-sm text-gray-500">{{ formatDate(u.created_at) }}</td>
              <td class="px-4 py-3">
                <div class="flex gap-2">
                  <RouterLink :to="`/users/${u.id}/edit`"
                    class="text-xs text-primary-600 hover:underline">Изменить</RouterLink>
                  <button
                    v-if="u.is_active"
                    class="text-xs text-red-500 hover:text-red-700"
                    @click="confirmDeactivate(u)">
                    Деактивировать
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- Pagination -->
        <div v-if="pagination.totalPages.value > 1" class="flex justify-between items-center px-4 py-3 border-t">
          <span class="text-sm text-gray-500">{{ pagination.page.value }} / {{ pagination.totalPages.value }}</span>
          <div class="flex gap-1">
            <button :disabled="!pagination.hasPrev.value"
              class="px-3 py-1 border rounded text-sm disabled:opacity-40"
              @click="changePage(pagination.page.value - 1)">←</button>
            <button :disabled="!pagination.hasNext.value"
              class="px-3 py-1 border rounded text-sm disabled:opacity-40"
              @click="changePage(pagination.page.value + 1)">→</button>
          </div>
        </div>
      </template>
    </div>

    <!-- Deactivate confirm modal -->
    <BaseModal v-model="showDeactivateModal" title="Деактивировать пользователя">
      <p class="text-sm text-gray-600">
        Вы уверены, что хотите деактивировать
        <span class="font-medium">{{ target?.first_name }} {{ target?.last_name }}</span>?
        Пользователь потеряет доступ к системе.
      </p>
      <template #footer>
        <BaseButton variant="secondary" @click="showDeactivateModal = false">Отмена</BaseButton>
        <BaseButton variant="danger" :loading="deactivating" @click="doDeactivate">Деактивировать</BaseButton>
      </template>
    </BaseModal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authApi } from '@/api/auth'
import { usePagination } from '@/composables/usePagination'
import { formatDate } from '@/utils/format'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import BaseBadge from '@/components/ui/BaseBadge.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import LoadingSpinner from '@/components/ui/LoadingSpinner.vue'
import type { User, Role } from '@/types/auth'

const loading = ref(false)
const users = ref<User[]>([])
const total = ref(0)
const search = ref('')
const roleFilter = ref('')
const showDeactivateModal = ref(false)
const deactivating = ref(false)
const target = ref<User | null>(null)
const pagination = usePagination(20)

let debounceTimer: ReturnType<typeof setTimeout>

const cols = ['Сотрудник', 'Телефон', 'Роль', 'Статус', 'Создан', '']
const roles = [
  { value: 'doctor', label: 'Врач' },
  { value: 'coordinator', label: 'Координатор' },
  { value: 'admin', label: 'Администратор' },
]

function roleBadge(role: Role): 'green' | 'red' | 'yellow' | 'blue' | 'gray' | 'orange' {
  const map: Record<Role, 'green' | 'red' | 'yellow' | 'blue' | 'gray' | 'orange'> = {
    doctor: 'blue',
    coordinator: 'yellow',
    admin: 'orange',
    super_admin: 'red',
  }
  return map[role] ?? 'gray'
}

function debouncedLoad() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(load, 300)
}

async function load() {
  loading.value = true
  try {
    const { data } = await authApi.listUsers({
      limit: pagination.pageSize,
      offset: pagination.offset.value,
    })
    users.value = data.users ?? []
    total.value = data.total ?? 0
    pagination.total.value = total.value
  } finally {
    loading.value = false
  }
}

async function changePage(p: number) {
  pagination.goTo(p)
  await load()
}

function confirmDeactivate(u: User) {
  target.value = u
  showDeactivateModal.value = true
}

async function doDeactivate() {
  if (!target.value) return
  deactivating.value = true
  try {
    await authApi.deactivateUser(target.value.id)
    showDeactivateModal.value = false
    await load()
  } finally {
    deactivating.value = false
  }
}

onMounted(load)
</script>
