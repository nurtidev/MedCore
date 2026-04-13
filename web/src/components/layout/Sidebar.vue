<template>
  <aside class="w-64 bg-gray-900 text-white flex flex-col min-h-screen">
    <!-- Logo -->
    <div class="px-6 py-5 border-b border-gray-700">
      <span class="text-xl font-bold text-white">Med<span class="text-primary-400">Core</span></span>
      <p class="text-xs text-gray-400 mt-0.5">Digital Clinic Hub</p>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 px-3 py-4 space-y-0.5 overflow-y-auto">
      <RouterLink to="/dashboard" class="nav-item">
        <IconGrid class="h-5 w-5" />
        {{ $t('nav.dashboard') }}
      </RouterLink>

      <!-- Analytics — admin+ -->
      <template v-if="auth.isAdmin">
        <div class="pt-4 pb-1 px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
          {{ $t('nav.analytics') }}
        </div>
        <RouterLink to="/analytics/revenue" class="nav-item">
          <IconChart class="h-5 w-5" />
          {{ $t('nav.revenue') }}
        </RouterLink>
        <RouterLink to="/analytics/doctors" class="nav-item">
          <IconUsers class="h-5 w-5" />
          {{ $t('nav.doctors') }}
        </RouterLink>
        <RouterLink to="/analytics/schedule" class="nav-item">
          <IconCalendar class="h-5 w-5" />
          {{ $t('nav.schedule') }}
        </RouterLink>
      </template>

      <!-- Billing -->
      <div class="pt-4 pb-1 px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
        {{ $t('nav.billing') }}
      </div>
      <RouterLink to="/billing/invoices" class="nav-item">
        <IconDocument class="h-5 w-5" />
        {{ $t('nav.invoices') }}
      </RouterLink>
      <RouterLink v-if="auth.isAdmin" to="/billing/subscription" class="nav-item">
        <IconCreditCard class="h-5 w-5" />
        {{ $t('nav.subscription') }}
      </RouterLink>

      <!-- Admin only -->
      <template v-if="auth.isAdmin">
        <div class="pt-4 pb-1 px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
          Управление
        </div>
        <RouterLink to="/users" class="nav-item">
          <IconUserGroup class="h-5 w-5" />
          {{ $t('nav.users') }}
        </RouterLink>
        <RouterLink to="/integrations" class="nav-item">
          <IconPuzzle class="h-5 w-5" />
          {{ $t('nav.integrations') }}
        </RouterLink>
      </template>

      <!-- Lab results — all -->
      <RouterLink to="/lab-results" class="nav-item mt-1">
        <IconBeaker class="h-5 w-5" />
        {{ $t('nav.lab_results') }}
      </RouterLink>
    </nav>

    <!-- User footer -->
    <div class="px-4 py-4 border-t border-gray-700">
      <div class="flex items-center gap-3">
        <div class="h-8 w-8 rounded-full bg-primary-600 flex items-center justify-center text-sm font-semibold">
          {{ initials }}
        </div>
        <div class="flex-1 min-w-0">
          <p class="text-sm font-medium text-white truncate">{{ auth.fullName }}</p>
          <p class="text-xs text-gray-400 truncate">{{ $t(`roles.${auth.user?.role}`) }}</p>
        </div>
        <button class="text-gray-400 hover:text-white transition" :title="$t('nav.logout')" @click="handleLogout">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h6a2 2 0 012 2v1"/>
          </svg>
        </button>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Inline icon components (Heroicons-style)
const IconGrid = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/></svg>' }
const IconChart = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>' }
const IconUsers = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>' }
const IconCalendar = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>' }
const IconDocument = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/></svg>' }
const IconCreditCard = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z"/></svg>' }
const IconUserGroup = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/></svg>' }
const IconPuzzle = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z"/></svg>' }
const IconBeaker = { template: '<svg fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v6l-3 8a2 2 0 001.9 2.7h8.2A2 2 0 0018 17l-3-8V3M9 3h6M9 3a1 1 0 00-1 1M15 3a1 1 0 011 1"/></svg>' }

const auth = useAuthStore()
const router = useRouter()

const initials = computed(() => {
  if (!auth.user) return '?'
  return `${auth.user.first_name[0]}${auth.user.last_name[0]}`.toUpperCase()
})

async function handleLogout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
@reference "../../style.css";

.nav-item {
  @apply flex items-center gap-3 px-3 py-2 rounded-lg text-sm text-gray-300
         hover:bg-gray-800 hover:text-white transition-colors;
}
.nav-item.router-link-active {
  @apply bg-primary-700 text-white;
}
</style>
