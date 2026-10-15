<script setup lang="ts">
import { computed } from 'vue'

type ConnectionStatus = 'disconnected' | 'searching' | 'connected'

const props = defineProps<{
  status: ConnectionStatus
  deviceName: string
  isDark: boolean
}>()

const emit = defineEmits<{
  cycleStatus: []
  toggleTheme: []
}>()

const pill = computed(() => {
  switch (props.status) {
    case 'searching':
      return {
        label: 'Searching',
        pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500 animate-pulse',
        animate: 'animate-pulse',
      }
    case 'connected':
      return {
        label: `Connected: ${props.deviceName}`,
        pill: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        dot: 'bg-emerald-500',
        animate: '',
      }
    default:
      return {
        label: 'Disconnected',
        pill: 'border-zinc-400/40 bg-zinc-500/10 text-zinc-600 dark:text-zinc-400',
        dot: 'bg-zinc-400 dark:bg-zinc-500',
        animate: '',
      }
  }
})
</script>

<template>
  <header
    class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-4 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div class="flex items-center gap-2">
      <svg
        class="h-5 w-5 text-blue-500"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <circle cx="9" cy="10" r="1.5" />
        <path d="m21 16-5-5-9 8" />
      </svg>
      <h1 class="text-sm font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
        Photo Bridge
      </h1>
    </div>

    <div class="flex items-center gap-3">
      <button
        type="button"
        class="flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        :class="pill.pill"
        title="Click to preview the next connection state"
        :aria-label="`Connection status: ${pill.label}. Click to preview the next state.`"
        @click="emit('cycleStatus')"
      >
        <span class="h-2 w-2 rounded-full" :class="pill.dot" />
        <span :class="pill.animate">{{ pill.label }}</span>
      </button>

      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-md text-zinc-600 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-300 dark:hover:bg-zinc-800"
        :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
        :title="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
        @click="emit('toggleTheme')"
      >
        <svg
          v-if="isDark"
          class="h-5 w-5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="4" />
          <path
            d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"
          />
        </svg>
        <svg
          v-else
          class="h-5 w-5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
        </svg>
      </button>
    </div>
  </header>
</template>
