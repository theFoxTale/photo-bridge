<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useComparisonStore } from '../../stores/comparisonStore'
import { useTransferStore } from '../../stores/transferStore'
import type { ConflictResolution } from '../../types/compare'

const comparisonStore = useComparisonStore()
const transferStore = useTransferStore()
const { counts, conflictResolution, selectedRows, isRunning, isStale } =
  storeToRefs(comparisonStore)
const {
  isRunning: isTransferring,
  isCancelling: isCancellingTransfer,
  progress: transferProgress,
  error: transferError,
  options: transferOptions,
} = storeToRefs(transferStore)

const resolutions: readonly { value: ConflictResolution; label: string }[] = [
  { value: 'skip', label: 'Skip' },
  { value: 'overwrite', label: 'Overwrite local' },
  { value: 'keepBoth', label: 'Keep both (suffix _1)' },
]

const busy = computed(() => isRunning.value || isTransferring.value)
const skipping = computed(() => conflictResolution.value === 'skip')

// Replacing a local file only works when the original is saved.
const overwriteBlocked = computed(
  () => conflictResolution.value === 'overwrite' && !transferOptions.value.saveOriginal,
)

const conflictCount = computed(() => (skipping.value ? 0 : counts.value.conflicts))
const newAndConflictCount = computed(() => counts.value.new + conflictCount.value)

const selectedCount = computed(
  () =>
    selectedRows.value.filter(
      (row) => row.photo !== null && !(row.status === 'conflict' && skipping.value),
    ).length,
)
const selectedHasConflicts = computed(() =>
  selectedRows.value.some((row) => row.status === 'conflict'),
)

const transferPercent = computed(() => {
  const p = transferProgress.value
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.round((p.completed / p.total) * 100))
})

const canTransferNew = computed(() => !busy.value && counts.value.new > 0)
const canTransferWithConflicts = computed(
  () => !busy.value && !skipping.value && !overwriteBlocked.value && newAndConflictCount.value > 0 && counts.value.conflicts > 0,
)
const canTransferSelected = computed(
  () =>
    !busy.value &&
    selectedCount.value > 0 &&
    !(overwriteBlocked.value && selectedHasConflicts.value),
)

const buttonClass =
  'rounded-md px-3 py-2 text-sm font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50'
</script>

<template>
  <div class="shrink-0 space-y-3 border-t border-zinc-200 bg-white px-4 py-3 dark:border-zinc-800 dark:bg-zinc-900">
    <div
      v-if="isStale"
      class="flex items-center justify-between gap-3 rounded-md border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-800 dark:text-amber-200"
      role="status"
    >
      <span>Files were transferred after this comparison, so the results are out of date.</span>
      <button
        type="button"
        class="shrink-0 font-semibold underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50"
        :disabled="busy"
        @click="comparisonStore.runComparison()"
      >
        Compare again
      </button>
    </div>

    <div v-if="isTransferring" class="space-y-1.5" role="status" aria-live="polite">
      <div class="flex items-center justify-between gap-3 text-xs text-zinc-600 dark:text-zinc-300">
        <span class="tabular-nums">
          {{ isCancellingTransfer ? 'Cancelling…' : 'Transferring' }}
          {{ transferProgress ? `${transferProgress.completed} of ${transferProgress.total} files` : '' }}
        </span>
        <button
          type="button"
          class="font-semibold underline focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50"
          :disabled="isCancellingTransfer"
          @click="transferStore.cancelTransfer()"
        >
          Cancel transfer
        </button>
      </div>
      <div
        class="h-1.5 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
        role="progressbar"
        aria-label="Transfer progress"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="transferPercent"
      >
        <div
          class="h-full rounded-full bg-blue-500 transition-[width] duration-300"
          :style="{ width: `${transferPercent}%` }"
        />
      </div>
    </div>

    <p v-if="transferError" class="text-xs text-red-500" role="alert">{{ transferError }}</p>

    <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <button
        type="button"
        :class="[buttonClass, 'bg-blue-600 text-white hover:bg-blue-500']"
        :disabled="!canTransferNew"
        @click="comparisonStore.transferNew()"
      >
        Transfer new only ({{ counts.new }})
      </button>

      <button
        type="button"
        :class="[
          buttonClass,
          'border border-zinc-300 text-zinc-800 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800',
        ]"
        :disabled="!canTransferWithConflicts"
        :title="
          skipping
            ? 'Conflicts are set to Skip, so this would only transfer the new photos'
            : undefined
        "
        @click="comparisonStore.transferNewAndConflicts()"
      >
        Transfer new + conflicts ({{ newAndConflictCount }})
      </button>

      <button
        type="button"
        :class="[
          buttonClass,
          'border border-zinc-300 text-zinc-800 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800',
        ]"
        :disabled="!canTransferSelected"
        @click="comparisonStore.transferSelected()"
      >
        Transfer selected ({{ selectedCount }})
      </button>

      <div class="ml-auto flex items-center gap-2">
        <label
          for="conflict-resolution"
          class="text-xs font-medium text-zinc-700 dark:text-zinc-300"
        >
          Conflicts
        </label>
        <select
          id="conflict-resolution"
          v-model="conflictResolution"
          :disabled="isTransferring"
          class="rounded-md border border-zinc-300 bg-white px-2 py-1.5 text-sm text-zinc-900 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
        >
          <option v-for="option in resolutions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </div>
    </div>

    <p v-if="overwriteBlocked" class="text-xs text-amber-600 dark:text-amber-400" role="alert">
      “Overwrite local” needs the Original format. Choose “Original only” or “Original + JPG” in the
      Transfer panel.
    </p>
    <p
      v-else-if="conflictResolution === 'overwrite' && counts.conflicts > 0"
      class="text-xs text-zinc-500 dark:text-zinc-400"
    >
      Conflicting local files will be replaced by the iPhone version.
    </p>
  </div>
</template>
