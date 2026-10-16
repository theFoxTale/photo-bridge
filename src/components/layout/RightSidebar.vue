<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { usePhotoLibrary } from '../../composables/usePhotoLibrary'

type TransferFormat = 'original' | 'jpg' | 'both'

const props = defineProps<{
  busy: boolean
}>()

const { selectedIds } = usePhotoLibrary()
const selectedCount = computed(() => selectedIds.value.size)

const emit = defineEmits<{
  transfer: [payload: { destination: string; format: TransferFormat; quality: number }]
}>()

const formatOptions: readonly { value: TransferFormat; label: string }[] = [
  { value: 'original', label: 'Original only' },
  { value: 'jpg', label: 'JPG only' },
  { value: 'both', label: 'Original + JPG' },
]

const destination = ref('')
const format = ref<TransferFormat>('original')
const quality = ref(95)
const destinationError = ref<string | null>(null)

const qualityEnabled = computed(() => format.value !== 'original')
const canTransfer = computed(() => selectedIds.value.size > 0 && !props.busy)

watch(destination, () => {
  destinationError.value = null
})

function onTransfer(): void {
  if (!canTransfer.value) return
  const trimmed = destination.value.trim()
  if (trimmed === '') {
    destinationError.value = 'Enter a destination folder.'
    return
  }
  destinationError.value = null
  emit('transfer', { destination: trimmed, format: format.value, quality: quality.value })
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
        <label
          for="destination"
          class="mb-1.5 block text-xs font-medium text-zinc-700 dark:text-zinc-300"
        >
          Destination folder
        </label>
        <input
          id="destination"
          v-model="destination"
          type="text"
          autocomplete="off"
          spellcheck="false"
          placeholder="C:\Photos\iPhone"
          class="w-full rounded-md border bg-white px-3 py-2 text-sm text-zinc-900 placeholder-zinc-400 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-zinc-950 dark:text-zinc-100 dark:placeholder-zinc-600"
          :class="
            destinationError
              ? 'border-red-500'
              : 'border-zinc-300 dark:border-zinc-700'
          "
          :aria-invalid="destinationError ? 'true' : 'false'"
          :aria-describedby="destinationError ? 'destination-error' : undefined"
        />
        <p v-if="destinationError" id="destination-error" class="mt-1 text-xs text-red-500" role="alert">
          {{ destinationError }}
        </p>
      </div>

      <fieldset>
        <legend class="mb-1.5 text-xs font-medium text-zinc-700 dark:text-zinc-300">Format</legend>
        <div class="space-y-1">
          <label
            v-for="option in formatOptions"
            :key="option.value"
            class="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm text-zinc-800 hover:bg-zinc-100 dark:text-zinc-200 dark:hover:bg-zinc-800"
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
          <span class="text-xs tabular-nums text-zinc-500 dark:text-zinc-400">{{ quality }}</span>
        </div>
        <input
          id="quality"
          v-model.number="quality"
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
        <button
          type="submit"
          :disabled="!canTransfer"
          class="w-full rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400 focus-visible:ring-offset-2 focus-visible:ring-offset-white disabled:cursor-not-allowed disabled:bg-zinc-200 disabled:text-zinc-400 dark:focus-visible:ring-offset-zinc-900 dark:disabled:bg-zinc-800 dark:disabled:text-zinc-500"
        >
          Transfer Selected{{ selectedCount > 0 ? ` (${selectedCount})` : '' }}
        </button>
      </div>
    </form>
  </aside>
</template>
