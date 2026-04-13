<template>
  <div class="max-w-md mx-auto">
    <div class="bg-white rounded-xl border p-6">
      <h2 class="text-lg font-semibold text-gray-900 mb-6">Изменить пароль</h2>

      <form v-if="!success" class="space-y-4" @submit.prevent="submit">
        <BaseInput
          v-model="form.oldPassword"
          type="password"
          label="Текущий пароль"
          required
          autocomplete="current-password"
        />
        <BaseInput
          v-model="form.newPassword"
          type="password"
          label="Новый пароль"
          required
          minlength="8"
          autocomplete="new-password"
        />
        <BaseInput
          v-model="form.confirmPassword"
          type="password"
          label="Подтвердите новый пароль"
          required
          autocomplete="new-password"
        />

        <p v-if="error" class="text-sm text-red-600">{{ error }}</p>

        <div class="flex gap-3 pt-2">
          <BaseButton type="submit" :loading="saving">Изменить пароль</BaseButton>
          <RouterLink to="/dashboard">
            <BaseButton variant="secondary">Отмена</BaseButton>
          </RouterLink>
        </div>
      </form>

      <div v-else class="text-center py-4">
        <div class="text-4xl mb-3">✅</div>
        <p class="text-gray-700 font-medium">Пароль успешно изменён</p>
        <RouterLink to="/dashboard">
          <BaseButton class="mt-4">На главную</BaseButton>
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { authApi } from '@/api/auth'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseInput from '@/components/ui/BaseInput.vue'

const saving = ref(false)
const success = ref(false)
const error = ref('')

const form = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

async function submit() {
  error.value = ''
  if (form.value.newPassword !== form.value.confirmPassword) {
    error.value = 'Пароли не совпадают'
    return
  }
  if (form.value.newPassword.length < 8) {
    error.value = 'Пароль должен содержать минимум 8 символов'
    return
  }
  saving.value = true
  try {
    await authApi.changePassword(form.value.oldPassword, form.value.newPassword)
    success.value = true
  } catch (e: any) {
    error.value = e?.response?.data?.error ?? 'Не удалось изменить пароль. Проверьте текущий пароль.'
  } finally {
    saving.value = false
  }
}
</script>
