<template>
  <div class="space-y-6">
    <!-- Current subscription -->
    <div v-if="billing.subscription" class="bg-white rounded-xl border p-6">
      <div class="flex items-start justify-between">
        <div>
          <h2 class="text-lg font-semibold text-gray-900">
            {{ billing.subscription.plan?.name ?? billing.subscription.plan_id }}
          </h2>
          <BaseBadge :variant="billing.subscription.status === 'active' ? 'green' : 'yellow'" class="mt-1">
            {{ billing.subscription.status }}
          </BaseBadge>
        </div>
        <div class="text-right text-sm text-gray-500">
          <p>До {{ formatDate(billing.subscription.current_period_end) }}</p>
        </div>
      </div>

      <div class="mt-6 grid grid-cols-2 gap-4">
        <ProgressBar
          label="Врачи"
          :value="billing.subscription.doctors_used ?? 0"
          :max="billing.subscription.plan?.max_doctors ?? 1"
        />
        <ProgressBar
          label="Пациенты"
          :value="billing.subscription.patients_used ?? 0"
          :max="billing.subscription.plan?.max_patients ?? 1"
        />
      </div>

      <div class="mt-4">
        <BaseButton variant="danger" size="sm" @click="showCancelModal = true">
          Отменить подписку
        </BaseButton>
      </div>
    </div>

    <!-- Plans grid -->
    <div>
      <h3 class="text-base font-semibold text-gray-900 mb-4">Тарифные планы</h3>
      <div v-if="billing.plans.length" class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div
          v-for="plan in billing.plans"
          :key="plan.id"
          :class="['bg-white rounded-xl border-2 p-5 transition',
            plan.tier === 'pro' ? 'border-primary-500 shadow-md' : 'border-gray-200']"
        >
          <div class="flex justify-between items-start mb-3">
            <div>
              <p class="font-semibold text-gray-900">{{ plan.name }}</p>
              <BaseBadge v-if="plan.tier === 'pro'" variant="blue" class="mt-1">Популярный</BaseBadge>
            </div>
            <div class="text-right">
              <p class="text-2xl font-bold text-gray-900">{{ formatKZT(plan.price_monthly) }}</p>
              <p class="text-xs text-gray-400">/месяц</p>
            </div>
          </div>
          <ul class="space-y-1.5 text-sm text-gray-600 mb-4">
            <li>👩‍⚕️ До {{ plan.max_doctors }} врачей</li>
            <li>🧑‍🤝‍🧑 До {{ plan.max_patients }} пациентов</li>
            <li v-for="f in plan.features" :key="f">✓ {{ f }}</li>
          </ul>
          <BaseButton
            :variant="billing.subscription?.plan_id === plan.id ? 'ghost' : 'primary'"
            class="w-full"
            :disabled="billing.subscription?.plan_id === plan.id"
            @click="upgrade(plan.id)"
          >
            {{ billing.subscription?.plan_id === plan.id ? 'Текущий тариф' : 'Выбрать' }}
          </BaseButton>
        </div>
      </div>
      <LoadingSpinner v-else />
    </div>

    <!-- Cancel modal -->
    <BaseModal v-model="showCancelModal" title="Отменить подписку">
      <p class="text-sm text-gray-600">Вы уверены? Доступ сохранится до конца оплаченного периода.</p>
      <template #footer>
        <BaseButton variant="secondary" @click="showCancelModal = false">Отмена</BaseButton>
        <BaseButton variant="danger" :loading="cancelling" @click="cancelSub">Подтвердить</BaseButton>
      </template>
    </BaseModal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, defineComponent, h, computed } from 'vue'
import { useBillingStore } from '@/stores/billing'
import { billingApi } from '@/api/billing'
import { formatDate, formatKZT } from '@/utils/format'
import BaseBadge from '@/components/ui/BaseBadge.vue'
import BaseButton from '@/components/ui/BaseButton.vue'
import BaseModal from '@/components/ui/BaseModal.vue'
import LoadingSpinner from '@/components/ui/LoadingSpinner.vue'

const billing = useBillingStore()
const showCancelModal = ref(false)
const cancelling = ref(false)

const ProgressBar = defineComponent({
  props: ['label', 'value', 'max'],
  setup(p) {
    const pct = computed(() => p.max ? Math.min(100, Math.round(p.value / p.max * 100)) : 0)
    return () => h('div', [
      h('div', { class: 'flex justify-between text-xs text-gray-500 mb-1' }, [h('span', p.label), h('span', `${p.value}/${p.max}`)]),
      h('div', { class: 'h-2 bg-gray-100 rounded-full' }, [
        h('div', { class: 'h-full bg-primary-500 rounded-full', style: `width:${pct.value}%` }),
      ]),
    ])
  },
})

async function cancelSub() {
  if (!billing.subscription) return
  cancelling.value = true
  try {
    await billingApi.cancelSubscription(billing.subscription.id)
    await billing.fetchSubscription()
    showCancelModal.value = false
  } finally {
    cancelling.value = false
  }
}

async function upgrade(planId: string) {
  await billingApi.upgradeSubscription(planId)
  await billing.fetchSubscription()
}

onMounted(async () => {
  await Promise.all([billing.fetchSubscription(), billing.fetchPlans()])
})
</script>
