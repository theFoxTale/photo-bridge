<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useComparisonStore } from '../../stores/comparisonStore'
import { useDeviceStore } from '../../stores/deviceStore'
import { usePhotoStore } from '../../stores/photoStore'
import { useTransferStore } from '../../stores/transferStore'
import type { CompareProgress } from '../../types/compare'

type TransferFormat = 'original' | 'jpg' | 'both'

const photoStore = usePhotoStore()
const transferStore = useTransferStore()
const comparisonStore = useComparisonStore()
const deviceStore = useDeviceStore()
const { result, isRunning: isComparing, progress: compareProgress, error: compareError, counts } =
  storeToRefs(comparisonStore)
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

const showSyncStatus = computed(
  () =>
    destPath.value.trim() !== '' &&
    (result.value !== null || isComparing.value || compareError.value !== null),
)

const canSyncNew = computed(
  () => !isRunning.value && activeDeviceId.value !== null && counts.value.new > 0,
)

function compareStatusText(progress: CompareProgress | null): string {
  if (!progress) return 'Comparing…'
  switch (progress.phase) {
    case 'scanning':
      return `Scanning the folder… ${progress.scanned} found`
    case 'listing':
      return 'Reading the iPhone photo list…'
    case 'hashing':
      return progress.total > 0
        ? `Verifying matches… ${progress.scanned} of ${progress.total}`
        : 'Verifying matches…'
    default:
      return 'Comparing…'
  }
}

/** Copies every photo the comparison classified as new. Conflicts stay put. */
async function syncNewPhotos(): Promise<void> {
  if (!canSyncNew.value) return
  const ids = result.value?.newOnDevice.map((photo) => photo.id) ?? []
  await transferStore.startTransfer(ids)
}

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
        <button
          type="button"
          :disabled="isRunning || isChoosingFolder"
          aria-labelledby="destination-label"
          :title="destPath === '' ? 'Choose the destination folder' : destPath"
          class="flex w-full items-center gap-2 rounded-md border border-zinc-300 bg-white px-3 py-2 text-left text-sm text-zinc-900 transition-colors hover:bg-zinc-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100 dark:hover:bg-zinc-900"
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
      </div>

      <section
        v-if="showSyncStatus"
        class="space-y-2 rounded-md border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950"
        aria-label="Sync status"
      >
        <h3 class="text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
          Sync status
        </h3>

        <p
          v-if="isComparing"
          class="text-xs text-zinc-600 dark:text-zinc-300"
          role="status"
          aria-live="polite"
        >
          {{ compareStatusText(compareProgress) }}
        </p>

        <dl v-if="result" class="space-y-1 text-sm">
          <div class="flex items-center justify-between gap-3">
            <dt class="text-zinc-600 dark:text-zinc-300">Total on device</dt>
            <dd class="tabular-nums text-zinc-900 dark:text-zinc-100">{{ counts.totalOnDevice }}</dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-emerald-700 dark:text-emerald-400">Already on PC</dt>
            <dd class="tabular-nums text-emerald-700 dark:text-emerald-400">{{ counts.existing }}</dd>
          </div>
          <div class="flex items-center justify-between gap-3">
            <dt class="text-blue-700 dark:text-blue-400">New</dt>
            <dd class="tabular-nums text-blue-700 dark:text-blue-400">{{ counts.new }}</dd>
          </div>
          <div v-if="counts.conflicts > 0" class="flex items-center justify-between gap-3">
            <dt class="text-amber-700 dark:text-amber-400">Conflicts</dt>
            <dd class="tabular-nums text-amber-700 dark:text-amber-400">{{ counts.conflicts }}</dd>
          </div>
        </dl>

        <p v-if="compareError" class="text-xs text-red-500" role="alert">{{ compareError }}</p>
        <button
          v-if="compareError && !isComparing"
          type="button"
          class="text-xs font-medium text-blue-600 underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-blue-400"
          @click="transferStore.runBackgroundCompare()"
        >
          Try again
        </button>

        <button
          v-if="result"
          type="button"
          :disabled="!canSyncNew"
          class="w-full rounded-md bg-blue-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400 dark:disabled:bg-zinc-800 dark:disabled:text-zinc-500"
          @click="syncNewPhotos()"
        >
          Sync New Photos ({{ counts.new }})
        </button>
        <p v-if="result && counts.conflicts > 0" class="text-[11px] text-zinc-500 dark:text-zinc-400">
          Conflicts are left as they are. Select them in the grid and use Transfer Selected.
        </p>
      </section>

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
          class="w-full rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-800 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800"
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
