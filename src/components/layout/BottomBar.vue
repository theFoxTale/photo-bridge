<script setup lang="ts">
import { computed } from 'vue'

interface TransferProgress {
  done: number
  total: number
  speedMBps: number
  etaSeconds: number
}

const props = defineProps<{
  progress: TransferProgress | null
}>()

const percent = computed(() => {
  const p = props.progress
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((p.done / p.total) * 100)))
})

const summary = computed(() => {
  const p = props.progress
  if (!p) return ''
  const total = Math.max(0, Math.round(p.etaSeconds))
  const mm = String(Math.floor(total / 60)).padStart(2, '0')
  const ss = String(total % 60).padStart(2, '0')
  return `${p.done} of ${p.total} files · ${p.speedMBps.toFixed(1)} MB/s · ETA ${mm}:${ss}`
})
</script>

<template>
  <footer
    class="flex h-10 shrink-0 items-center border-t border-zinc-200 bg-white px-4 text-xs dark:border-zinc-800 dark:bg-zinc-900"
    aria-live="polite"
  >
    <div v-if="!progress" class="flex items-center gap-2 text-zinc-500 dark:text-zinc-400">
      <span class="h-2 w-2 rounded-full bg-zinc-400 dark:bg-zinc-600" />
      <span>Idle</span>
    </div>

    <div v-else class="flex w-full items-center gap-4">
      <div
        class="h-2 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
        role="progressbar"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="percent"
      >
        <div
          class="h-full rounded-full bg-blue-500 transition-[width] duration-300"
          :style="{ width: `${percent}%` }"
        />
      </div>
      <span class="shrink-0 tabular-nums text-zinc-700 dark:text-zinc-300">{{ summary }}</span>
    </div>
  </footer>
</template>
