<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useDeviceDiscovery } from '../../composables/useDeviceDiscovery'
import { useComparisonStore } from '../../stores/comparisonStore'

defineProps<{
  isDark: boolean
}>()

const emit = defineEmits<{
  toggleTheme: []
}>()

const { status, activeDevice, error, refresh, stopDiscovery } = useDeviceDiscovery()

const comparisonStore = useComparisonStore()
const { isRunning: isComparing, progress: compareProgress } = storeToRefs(comparisonStore)

// While a comparison runs the button turns into a progress indicator; it stays
// clickable so a closed comparison view can be reopened (and cancelled).
const comparePercent = computed(() => {
  const p = compareProgress.value
  if (!p || p.phase !== 'hashing' || p.total <= 0) return null
  return Math.min(100, Math.round((p.scanned / p.total) * 100))
})
const compareTitle = computed(() =>
  isComparing.value ? 'A comparison is running. Click to view it.' : 'Compare the iPhone with a folder on this PC',
)

const pill = computed(() => {
  switch (status.value) {
    case 'searching':
      return {
        label: 'Searching',
        pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500 animate-pulse',
        animate: 'animate-pulse',
      }
    case 'blocked_by_apple':
      return {
        label: 'Blocked by Apple Devices',
        pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500',
        animate: '',
      }
    case 'connected':
      return {
        label: activeDevice.value ? `Connected: ${activeDevice.value.name}` : 'Connected',
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

const pillTitle = computed(() => {
  if (status.value === 'searching') return 'Click to stop searching'
  if (status.value === 'blocked_by_apple') {
    return 'iPhone found on Wi-Fi, but Apple Devices cannot see it. Click to retry.'
  }
  if (status.value === 'error' && error.value) return `Discovery failed: ${error.value}. Click to retry.`
  return 'Click to search for devices'
})

async function onPillClick(): Promise<void> {
  if (status.value === 'searching') {
    await stopDiscovery()
    return
  }
  await refresh()
}
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
        class="flex items-center gap-2 rounded-md border border-zinc-300 px-3 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
        :title="compareTitle"
        :aria-label="compareTitle"
        @click="comparisonStore.open()"
      >
        <svg
          v-if="isComparing"
          class="h-4 w-4 animate-spin text-blue-500"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4z" />
        </svg>
        <svg
          v-else
          class="h-4 w-4"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M16 3h5v5M4 20 21 3M21 16v5h-5M15 15l6 6M4 4l5 5" />
        </svg>
        <span class="tabular-nums">
          {{ isComparing ? (comparePercent !== null ? `Comparing ${comparePercent}%` : 'Comparing…') : 'Compare folders' }}
        </span>
      </button>

      <button
        type="button"
        class="flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        :class="pill.pill"
        :title="pillTitle"
        :aria-label="`Connection status: ${pill.label}. ${pillTitle}`"
        @click="onPillClick"
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
