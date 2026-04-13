<template>
  <span :class="['inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium', colorClass]">
    <slot />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  variant?: 'green' | 'red' | 'yellow' | 'blue' | 'gray' | 'orange'
}>()

const colorClass = computed(() => ({
  'bg-green-100 text-green-800':  !props.variant || props.variant === 'green',
  'bg-red-100 text-red-800':      props.variant === 'red',
  'bg-yellow-100 text-yellow-800': props.variant === 'yellow',
  'bg-blue-100 text-blue-800':    props.variant === 'blue',
  'bg-gray-100 text-gray-700':    props.variant === 'gray',
  'bg-orange-100 text-orange-800': props.variant === 'orange',
}))
</script>
