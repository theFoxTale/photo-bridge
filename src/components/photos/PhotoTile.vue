<script setup lang="ts">
defineProps<{
  label: string
  selected: boolean
}>()

const emit = defineEmits<{
  toggle: []
}>()
</script>

<template>
  <button
    type="button"
    class="group relative aspect-square w-full overflow-hidden rounded-lg ring-2 transition focus:outline-none focus-visible:ring-blue-400"
    :class="selected ? 'ring-blue-500' : 'ring-transparent hover:ring-zinc-400 dark:hover:ring-zinc-600'"
    :aria-pressed="selected"
    :aria-label="`${selected ? 'Deselect' : 'Select'} ${label}`"
    @click="emit('toggle')"
  >
    <span
      class="absolute inset-0 bg-linear-to-br from-zinc-200 to-zinc-300 dark:from-zinc-800 dark:to-zinc-700"
    />
    <span
      class="absolute inset-0 flex items-center justify-center text-xs font-medium text-zinc-500 dark:text-zinc-400"
    >
      {{ label }}
    </span>
    <span
      class="absolute right-2 top-2 flex h-5 w-5 items-center justify-center rounded-full border text-white transition"
      :class="
        selected
          ? 'border-blue-500 bg-blue-500'
          : 'border-zinc-400 bg-white/70 opacity-0 group-hover:opacity-100 dark:border-zinc-500 dark:bg-zinc-900/70'
      "
      aria-hidden="true"
    >
      <svg
        v-if="selected"
        class="h-3 w-3"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="3"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M20 6 9 17l-5-5" />
      </svg>
    </span>
  </button>
</template>
