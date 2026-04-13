<template>
  <div class="max-w-lg">
    <div class="bg-white rounded-xl border p-6">
      <h2 class="text-lg font-semibold text-gray-900 mb-6">
        {{ isEdit ? 'Редактировать сотрудника' : 'Новый сотрудник' }}
      </h2>

      <div v-if="loadingUser" class="animate-pulse space-y-4">
        <div v-for="i in 5" :key="i" class="h-10 bg-gray-200 rounded" />
      </div>

      <form v-else class="space-y-4" @submit.prevent="submit">
        <div class="grid grid-cols-2 gap-4">
          <BaseInput v-model="form.first_name" label="Имя" required />
          <BaseInput v-model="form.last_name" label="Фамилия" required />
        </div>
        <BaseInput v-model="form.email" type="email" label="Email" :disabled="isEdit" required />
        <BaseInput v-model="form.phone" label="Телефон" placeholder="+7 (700) 000-00-00" />

        <div v-if="!isEdit">
          <label class="block text-sm font-medium text-gray-700 mb-1">Роль</label>
          <select
            v-model="form.role"
            required
            class="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
          >
            <option value="">Выберите роль</option>
            <option v-for="r in roles" :key="r.value" :value="r.value">{{ r.label }}</option>
          </select>
        </div>

        <div v-if="!isEdit">
          <BaseInput v-model="form.password" type="password" label="Пароль" required minlength="8" />
        </div>

        <p v-if="error" class="text-sm text-red-600">{{ error }}</p>

        <div class="flex gap-3 pt-2">
          <BaseButton type="submit" :loading="saving">
            {{ isEdit ? 'Сохранить' : 'Создать' }}
          </BaseButton>
          <RouterLink to="/users">
            <BaseButton variant="secondary">Отмена</BaseButton>
          </RouterLink>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '@/api/auth'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const isEdit = computed(() => !!route.params.id)
const loadingUser = ref(false)
const saving = ref(false)
const error = ref('')

const form = ref({
  first_name: '',
  last_name: '',
  email: '',
  phone: '',
  role: '',
  password: '',
})

const roles = [
  { value: 'doctor', label: 'Врач' },
  { value: 'coordinator', label: 'Координатор' },
  { value: 'admin', label: 'Администратор' },
]

onMounted(async () => {
  if (!isEdit.value) return
  loadingUser.value = true
  try {
    const { data } = await authApi.getUser(route.params.id as string)
    form.value.first_name = data.first_name
    form.value.last_name = data.last_name
    form.value.email = data.email
    form.value.phone = data.phone ?? ''
  } finally {
    loadingUser.value = false
  }
})

async function submit() {
  saving.value = true
  error.value = ''
  try {
    if (isEdit.value) {
      await authApi.updateUser(route.params.id as string, {
        first_name: form.value.first_name,
        last_name: form.value.last_name,
        phone: form.value.phone || undefined,
      })
    } else {
      await authApi.register({
        clinic_id: auth.user!.clinic_id,
        email: form.value.email,
        password: form.value.password,
        first_name: form.value.first_name,
        last_name: form.value.last_name,
        phone: form.value.phone || undefined,
        role: form.value.role as any,
      })
    }
    router.push('/users')
  } catch (e: any) {
    error.value = e?.response?.data?.error ?? 'Произошла ошибка. Попробуйте ещё раз.'
  } finally {
    saving.value = false
  }
}
</script>
