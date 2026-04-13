<template>
  <button
    v-bind="$attrs"
    :disabled="disabled || loading"
    :class="[baseClass, variantClass, sizeClass, { 'opacity-60 cursor-not-allowed': disabled || loading }]"
  >
    <svg v-if="loading" class="animate-spin -ml-1 mr-2 h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
    </svg>
    <slot />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md' | 'lg'
  loading?: boolean
  disabled?: boolean
}>()

const baseClass = 'inline-flex items-center justify-center font-medium rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2'

const variantClass = computed(() => ({
  'bg-primary-600 text-white hover:bg-primary-700 focus:ring-primary-500': !props.variant || props.variant === 'primary',
  'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 focus:ring-primary-500': props.variant === 'secondary',
  'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500': props.variant === 'danger',
  'text-gray-600 hover:text-gray-900 hover:bg-gray-100 focus:ring-gray-500': props.variant === 'ghost',
}))

const sizeClass = computed(() => ({
  'px-3 py-1.5 text-sm': props.size === 'sm',
  'px-4 py-2 text-sm': !props.size || props.size === 'md',
  'px-6 py-3 text-base': props.size === 'lg',
}))
</script>
