<template>
  <div class="flex items-center justify-center" :class="containerClass">
    <svg class="animate-spin" :class="sizeClass" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ size?: 'sm' | 'md' | 'lg'; fullPage?: boolean }>()
const sizeClass = computed(() => ({
  'h-4 w-4': props.size === 'sm',
  'h-8 w-8': !props.size || props.size === 'md',
  'h-12 w-12': props.size === 'lg',
}))
const containerClass = computed(() => props.fullPage ? 'min-h-screen text-primary-600' : 'text-primary-600')
</script>
