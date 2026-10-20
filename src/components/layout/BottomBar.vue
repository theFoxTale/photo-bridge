<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useTransferStore } from '../../stores/transferStore'
import { formatSpeed } from '../../utils/format'

const transferStore = useTransferStore()
const { isRunning, isCancelling, progress } = storeToRefs(transferStore)

const percent = computed(() => {
  const p = progress.value
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((p.completed / p.total) * 100)))
})

// "X of Y files · 12.4 MB/s ·" is followed by the (possibly long) file name.
const summary = computed(() => {
  const p = progress.value
  if (!p) return ''
  return `${p.completed} of ${p.total} files · ${formatSpeed(p.bytesPerSec)} ·`
})

const currentFile = computed(() => {
  if (isCancelling.value) return 'Cancelling…'
  return progress.value?.currentFile || 'Starting…'
})
</script>

<template>
  <footer
    class="flex h-10 shrink-0 items-center border-t border-zinc-200 bg-white px-4 text-xs dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div v-if="!isRunning || !progress" class="flex items-center gap-2 text-zinc-500 dark:text-zinc-400">
      <span class="h-2 w-2 rounded-full bg-zinc-400 dark:bg-zinc-600" />
      <span>Idle</span>
    </div>

    <div v-else class="flex w-full items-center gap-4">
      <div
        class="h-2 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
        role="progressbar"
        aria-label="Transfer progress"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="percent"
      >
        <div
          class="h-full rounded-full bg-blue-500 transition-[width] duration-300"
          :style="{ width: `${percent}%` }"
        />
      </div>
      <div class="flex min-w-0 max-w-[55%] shrink-0 items-center gap-1.5 text-zinc-700 dark:text-zinc-300">
        <span class="shrink-0 tabular-nums">{{ summary }}</span>
        <span class="min-w-0 truncate" :title="currentFile">{{ currentFile }}</span>
      </div>
    </div>
  </footer>
</template>
