<template>
  <div>
    <label v-if="label" :for="id" class="block text-sm font-medium text-gray-700 mb-1">
      {{ label }}
    </label>
    <input
      :id="id"
      v-bind="$attrs"
      :value="modelValue"
      :type="type"
      :placeholder="placeholder"
      :disabled="disabled"
      :class="[
        'block w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition',
        'focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500',
        error ? 'border-red-300 bg-red-50' : 'border-gray-300 bg-white',
        disabled ? 'opacity-60 cursor-not-allowed bg-gray-50' : '',
      ]"
      @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <p v-if="error" class="mt-1 text-xs text-red-600">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  id?: string
  label?: string
  modelValue?: string
  type?: string
  placeholder?: string
  disabled?: boolean
  error?: string
}>()
defineEmits(['update:modelValue'])
</script>
