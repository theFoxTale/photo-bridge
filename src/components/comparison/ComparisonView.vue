<script setup lang="ts">
import { onKeyStroke } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { computed, nextTick, ref, watch } from 'vue'
import { useComparisonStore } from '../../stores/comparisonStore'
import { useDeviceStore } from '../../stores/deviceStore'
import { formatDateTime } from '../../utils/format'
import ComparisonActions from './ComparisonActions.vue'
import ComparisonTable from './ComparisonTable.vue'

const comparisonStore = useComparisonStore()
const deviceStore = useDeviceStore()
const {
  isOpen,
  isRunning,
  isCancelling,
  isChoosingFolder,
  wasCancelled,
  result,
  localPath,
  comparedPath,
  progress,
  error,
} = storeToRefs(comparisonStore)
const { activeDeviceId, activeDevice } = storeToRefs(deviceStore)

const dialog = ref<HTMLElement | null>(null)

const canRun = computed(
  () => !isRunning.value && activeDeviceId.value !== null && localPath.value.trim() !== '',
)

const percent = computed(() => {
  const p = progress.value
  if (!p || p.phase !== 'hashing' || p.total <= 0) return null
  return Math.min(100, Math.round((p.scanned / p.total) * 100))
})

const progressLabel = computed(() => {
  const p = progress.value
  if (isCancelling.value) return 'Cancelling…'
  if (!p) return 'Starting…'
  switch (p.phase) {
    case 'scanning':
      return `Scanning the folder… ${p.scanned} ${p.scanned === 1 ? 'image' : 'images'} found`
    case 'listing':
      return 'Reading the photo list from the iPhone…'
    case 'hashing':
      return `Verifying matches… ${p.scanned} of ${p.total}`
    default:
      return 'Finishing…'
  }
})

onKeyStroke('Escape', () => {
  if (isOpen.value) comparisonStore.close()
})

// Move focus into the dialog so the keyboard works at once.
watch(isOpen, async (open) => {
  if (!open) return
  await nextTick()
  dialog.value?.focus()
})
</script>

<template>
  <Teleport to="body">
    <!-- Leaves the 40px bottom bar visible so transfer progress stays in view. -->
    <div
      v-if="isOpen"
      ref="dialog"
      class="fixed inset-x-0 top-0 bottom-10 z-40 flex flex-col bg-zinc-50 text-zinc-900 focus:outline-none dark:bg-zinc-950 dark:text-zinc-100"
      role="dialog"
      aria-modal="true"
      aria-labelledby="comparison-title"
      tabindex="-1"
    >
      <header
        class="flex shrink-0 items-center justify-between gap-4 border-b border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900"
      >
        <div class="min-w-0">
          <h2 id="comparison-title" class="text-sm font-semibold">Compare iPhone with a folder</h2>
          <p class="truncate text-xs text-zinc-500 dark:text-zinc-400">
            {{ activeDevice ? activeDevice.name : 'No device connected' }}
          </p>
        </div>
        <button
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-md text-zinc-600 hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-300 dark:hover:bg-zinc-800"
          aria-label="Close comparison"
          title="Close (Esc)"
          @click="comparisonStore.close()"
        >
          <svg
            class="h-5 w-5"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            aria-hidden="true"
          >
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </header>

      <div
        class="flex shrink-0 flex-wrap items-center gap-3 border-b border-zinc-200 px-4 py-3 dark:border-zinc-800"
      >
        <span id="compare-folder-label" class="text-xs font-medium text-zinc-700 dark:text-zinc-300">
          Folder
        </span>
        <button
          type="button"
          :disabled="isRunning || isChoosingFolder"
          aria-labelledby="compare-folder-label"
          :title="localPath === '' ? 'Choose the folder to compare with' : localPath"
          class="flex min-w-0 max-w-xl flex-1 items-center gap-2 rounded-md border border-zinc-300 bg-white px-3 py-2 text-left text-sm hover:bg-zinc-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:hover:bg-zinc-900"
          @click="comparisonStore.chooseLocalFolder()"
        >
          <svg
            class="h-4 w-4 shrink-0 text-zinc-500 dark:text-zinc-400"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
          </svg>
          <span
            class="min-w-0 flex-1 truncate"
            :class="localPath === '' ? 'text-zinc-400 dark:text-zinc-500' : ''"
          >
            {{ isChoosingFolder ? 'Opening…' : localPath === '' ? 'Choose folder…' : localPath }}
          </span>
        </button>

        <button
          v-if="!isRunning"
          type="button"
          :disabled="!canRun"
          :title="activeDeviceId === null ? 'Connect an iPhone first' : undefined"
          class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400 dark:disabled:bg-zinc-800 dark:disabled:text-zinc-500"
          @click="comparisonStore.runComparison()"
        >
          {{ result ? 'Compare again' : 'Compare' }}
        </button>
        <button
          v-else
          type="button"
          :disabled="isCancelling"
          class="rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
          @click="comparisonStore.cancelComparison()"
        >
          {{ isCancelling ? 'Cancelling…' : 'Cancel' }}
        </button>
      </div>

      <!-- Loading -->
      <div
        v-if="isRunning"
        class="flex flex-1 flex-col items-center justify-center gap-4 p-8"
        role="status"
        aria-live="polite"
      >
        <svg
          class="h-8 w-8 animate-spin text-blue-500"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 0 1 8-8v4a4 4 0 0 0-4 4z" />
        </svg>
        <p class="text-sm font-medium tabular-nums">{{ progressLabel }}</p>
        <div
          class="h-2 w-full max-w-md overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
          role="progressbar"
          aria-label="Comparison progress"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="percent ?? undefined"
          :aria-busy="percent === null"
        >
          <div
            v-if="percent !== null"
            class="h-full rounded-full bg-blue-500 transition-[width] duration-300"
            :style="{ width: `${percent}%` }"
          />
          <div v-else class="h-full w-1/3 animate-pulse rounded-full bg-blue-500" />
        </div>
        <p class="max-w-md text-center text-xs text-zinc-500 dark:text-zinc-400">
          Only photos with the same date and size are read in full, and results are remembered for
          the next run.
        </p>
      </div>

      <!-- Error -->
      <div
        v-else-if="error"
        class="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
        role="alert"
      >
        <p class="text-sm font-medium text-red-500">The comparison failed</p>
        <p class="max-w-md text-sm text-zinc-600 dark:text-zinc-300">{{ error }}</p>
        <button
          v-if="canRun"
          type="button"
          class="rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-zinc-700 dark:hover:bg-zinc-800"
          @click="comparisonStore.runComparison()"
        >
          Try again
        </button>
      </div>

      <!-- Cancelled -->
      <div
        v-else-if="wasCancelled"
        class="flex flex-1 flex-col items-center justify-center gap-2 p-8 text-center"
        role="status"
      >
        <p class="text-sm font-medium">Comparison cancelled</p>
        <p class="text-xs text-zinc-500 dark:text-zinc-400">
          Nothing was changed. Press Compare to start again; files already checked are remembered.
        </p>
      </div>

      <!-- Result -->
      <template v-else-if="result">
        <p class="shrink-0 px-4 py-2 text-xs text-zinc-500 dark:text-zinc-400">
          Compared with
          <span class="font-medium text-zinc-700 dark:text-zinc-300" :title="comparedPath">{{
            comparedPath
          }}</span>
          · {{ formatDateTime(result.scannedAt) }}
        </p>
        <ComparisonTable />
        <ComparisonActions />
      </template>

      <!-- Nothing run yet -->
      <div
        v-else
        class="flex flex-1 flex-col items-center justify-center gap-2 p-8 text-center"
      >
        <p class="text-sm font-medium">
          {{ activeDeviceId === null ? 'Connect an iPhone to compare' : 'Compare the iPhone with a folder' }}
        </p>
        <p class="max-w-md text-xs text-zinc-500 dark:text-zinc-400">
          {{
            activeDeviceId === null
              ? 'No device is connected. Connect and unlock your iPhone, then try again.'
              : 'Choose a folder and press Compare to see which photos are new on the iPhone, already on this PC, in conflict, or only on the PC.'
          }}
        </p>
      </div>
    </div>
  </Teleport>
</template>
