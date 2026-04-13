<template>
  <div class="min-h-screen bg-gradient-to-br from-primary-900 to-primary-700 flex items-center justify-center p-4">
    <div class="bg-white rounded-2xl shadow-2xl w-full max-w-md p-8">
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-gray-900">Med<span class="text-primary-600">Core</span></h1>
        <p class="text-gray-500 mt-2 text-sm">Digital Clinic Hub</p>
      </div>

      <form @submit.prevent="handleSubmit">
        <div class="space-y-4">
          <BaseInput
            id="email"
            v-model="email"
            type="email"
            :label="$t('auth.email')"
            placeholder="doctor@clinic.kz"
            :error="errors.email"
            autocomplete="email"
          />
          <BaseInput
            id="password"
            v-model="password"
            type="password"
            :label="$t('auth.password')"
            :error="errors.password"
            autocomplete="current-password"
          />
        </div>

        <p v-if="serverError" class="mt-4 text-sm text-red-600 text-center">{{ serverError }}</p>

        <BaseButton type="submit" class="w-full mt-6" size="lg" :loading="loading">
          {{ $t('auth.sign_in') }}
        </BaseButton>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import BaseInput from '@/components/ui/BaseInput.vue'
import BaseButton from '@/components/ui/BaseButton.vue'

const router = useRouter()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const loading = ref(false)
const serverError = ref('')
const errors = reactive({ email: '', password: '' })

function validate(): boolean {
  errors.email = ''
  errors.password = ''
  if (!email.value) { errors.email = 'Введите email'; return false }
  if (!password.value) { errors.password = 'Введите пароль'; return false }
  return true
}

async function handleSubmit() {
  if (!validate()) return
  loading.value = true
  serverError.value = ''
  try {
    await auth.login(email.value, password.value)
    router.push('/dashboard')
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    serverError.value = err.response?.data?.message ?? 'Неверный email или пароль'
  } finally {
    loading.value = false
  }
}
</script>
