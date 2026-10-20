<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useComparisonStore } from '../../stores/comparisonStore'
import { useDeviceStore } from '../../stores/deviceStore'
import { usePhotoStore } from '../../stores/photoStore'
import { useTransferStore } from '../../stores/transferStore'

type TransferFormat = 'original' | 'jpg' | 'both'

const photoStore = usePhotoStore()
const transferStore = useTransferStore()
const comparisonStore = useComparisonStore()
const deviceStore = useDeviceStore()
const { isRunning: isComparing } = storeToRefs(comparisonStore)
const { activeDeviceId } = storeToRefs(deviceStore)
const { selectedIds } = storeToRefs(photoStore)
const { options, destPath, isRunning, isCancelling, isChoosingFolder, error } =
  storeToRefs(transferStore)

const selectedCount = computed(() => selectedIds.value.size)

const formatOptions: readonly { value: TransferFormat; label: string }[] = [
  { value: 'original', label: 'Original only' },
  { value: 'jpg', label: 'JPG only' },
  { value: 'both', label: 'Original + JPG' },
]

// The three radio buttons map onto the two booleans of the transfer options.
const format = computed<TransferFormat>({
  get: () => {
    if (options.value.saveOriginal && options.value.saveJpg) return 'both'
    return options.value.saveJpg ? 'jpg' : 'original'
  },
  set: (value) => {
    options.value = {
      ...options.value,
      saveOriginal: value !== 'jpg',
      saveJpg: value !== 'original',
    }
  },
})

const qualityEnabled = computed(() => options.value.saveJpg && !isRunning.value)
const canTransfer = computed(() => selectedCount.value > 0 && !isRunning.value)

const canCompare = computed(
  () =>
    !isComparing.value &&
    !isRunning.value &&
    activeDeviceId.value !== null &&
    destPath.value.trim() !== '',
)
const compareTitle = computed(() => {
  if (isComparing.value) return 'Comparison in progress…'
  if (activeDeviceId.value === null) return 'Connect an iPhone to compare'
  if (destPath.value.trim() === '') return 'Choose a destination folder to compare with'
  if (isRunning.value) return 'Wait for the transfer to finish'
  return 'Compare the iPhone with this folder'
})

async function onTransfer(): Promise<void> {
  if (!canTransfer.value) return
  await transferStore.startTransfer(Array.from(selectedIds.value))
}
</script>

<template>
  <aside
    class="flex w-[280px] shrink-0 flex-col border-l border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
    aria-label="Transfer"
  >
    <div class="border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
      <h2 class="text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
        Transfer
      </h2>
    </div>

    <form class="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-4" @submit.prevent="onTransfer">
      <div>
        <span
          id="destination-label"
          class="mb-1.5 block text-xs font-medium text-zinc-700 dark:text-zinc-300"
        >
          Destination folder
        </span>
        <div class="flex items-stretch gap-2">
        <button
          type="button"
          :disabled="isRunning || isChoosingFolder"
          aria-labelledby="destination-label"
          :title="destPath === '' ? 'Choose the destination folder' : destPath"
          class="flex min-w-0 flex-1 items-center gap-2 rounded-md border border-zinc-300 bg-white px-3 py-2 text-left text-sm text-zinc-900 transition-colors hover:bg-zinc-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100 dark:hover:bg-zinc-900"
          @click="transferStore.chooseDestFolder()"
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
            :class="destPath === '' ? 'text-zinc-400 dark:text-zinc-500' : ''"
          >
            {{ isChoosingFolder ? 'Opening…' : destPath === '' ? 'Choose folder…' : destPath }}
          </span>
        </button>
        <button
          type="button"
          :disabled="!canCompare"
          :title="compareTitle"
          :aria-label="compareTitle"
          class="flex w-9 shrink-0 items-center justify-center rounded-md border border-zinc-300 text-zinc-600 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          @click="comparisonStore.runComparison(destPath)"
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
        </button>
        </div>
      </div>

      <fieldset :disabled="isRunning">
        <legend class="mb-1.5 text-xs font-medium text-zinc-700 dark:text-zinc-300">Format</legend>
        <div class="space-y-1">
          <label
            v-for="option in formatOptions"
            :key="option.value"
            class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-zinc-800 hover:bg-zinc-100 dark:text-zinc-200 dark:hover:bg-zinc-800"
            :class="isRunning ? 'cursor-not-allowed opacity-60' : ''"
          >
            <input
              v-model="format"
              type="radio"
              name="format"
              :value="option.value"
              class="h-4 w-4 accent-blue-500"
            />
            {{ option.label }}
          </label>
        </div>
      </fieldset>

      <div :class="qualityEnabled ? '' : 'opacity-50'">
        <div class="mb-1.5 flex items-center justify-between">
          <label for="quality" class="text-xs font-medium text-zinc-700 dark:text-zinc-300">
            JPG quality
          </label>
          <span class="text-xs tabular-nums text-zinc-500 dark:text-zinc-400">
            {{ options.jpgQuality }}
          </span>
        </div>
        <input
          id="quality"
          v-model.number="options.jpgQuality"
          type="range"
          min="80"
          max="100"
          step="1"
          :disabled="!qualityEnabled"
          class="w-full accent-blue-500 disabled:cursor-not-allowed"
        />
        <div class="mt-1 flex justify-between text-[10px] text-zinc-400 dark:text-zinc-500">
          <span>80</span>
          <span>100</span>
        </div>
      </div>

      <div class="mt-auto space-y-2">
        <p class="text-center text-xs text-zinc-500 dark:text-zinc-400" aria-live="polite">
          {{ selectedCount === 0 ? 'No files selected' : `${selectedCount} selected` }}
        </p>
        <p v-if="error" class="text-xs text-red-500" role="alert">{{ error }}</p>
        <button
          type="submit"
          :disabled="!canTransfer"
          class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-2 focus-visible:ring-offset-white disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400 dark:focus-visible:ring-offset-zinc-900 dark:disabled:bg-zinc-800 dark:disabled:text-zinc-500"
        >
          {{
            isRunning
              ? 'Transferring…'
              : `Transfer Selected${selectedCount > 0 ? ` (${selectedCount})` : ''}`
          }}
        </button>
        <button
          v-if="isRunning"
          type="button"
          :disabled="isCancelling"
          class="w-full rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-200 dark:hover:bg-zinc-800"
          @click="transferStore.cancelTransfer()"
        >
          {{ isCancelling ? 'Cancelling…' : 'Cancel' }}
        </button>
      </div>
    </form>
  </aside>
</template>
