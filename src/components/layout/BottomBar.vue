<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDeviceStore } from '../../stores/deviceStore'
import { useSystemStore } from '../../stores/systemStore'
import { useTransferStore } from '../../stores/transferStore'
import { formatBytes, formatSpeed } from '../../utils/format'

const { t } = useI18n()
const transferStore = useTransferStore()
const deviceStore = useDeviceStore()
const systemStore = useSystemStore()
const { isRunning, isCancelling, progress, destPath } = storeToRefs(transferStore)
const { activeDeviceId } = storeToRefs(deviceStore)
const { pcFree, iPhoneFree } = storeToRefs(systemStore)

const percent = computed(() => {
  const p = progress.value
  if (!p || p.total <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((p.completed / p.total) * 100)))
})

// "X of Y files · 12.4 MB/s ·" is followed by the (possibly long) file name.
const summary = computed(() => {
  const p = progress.value
  if (!p) return ''
  return t('progress.summary', {
    completed: p.completed,
    total: p.total,
    speed: formatSpeed(p.bytesPerSec),
  })
})

const currentFile = computed(() => {
  if (isCancelling.value) return t('progress.cancelling')
  return progress.value?.currentFile || t('progress.starting')
})

const showPc = computed(() => destPath.value.trim() !== '' && pcFree.value !== null)
const showPhone = computed(
  () => activeDeviceId.value !== null && iPhoneFree.value !== null && iPhoneFree.value > 0,
)
const pcLabel = computed(() => t('bottom.pcFree', { size: formatBytes(pcFree.value ?? 0) }))
const phoneLabel = computed(() =>
  t('bottom.iphoneFree', { size: formatBytes(iPhoneFree.value ?? 0) }),
)
</script>

<template>
  <footer
    class="flex h-10 shrink-0 items-center justify-between gap-4 border-t border-zinc-200 bg-white px-4 text-xs dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div v-if="!isRunning || !progress" class="flex min-w-0 items-center gap-2 text-zinc-500 dark:text-zinc-400">
      <span class="h-2 w-2 shrink-0 rounded-full bg-zinc-400 dark:bg-zinc-600" />
      <span>{{ $t('progress.idle') }}</span>
    </div>

    <div v-else class="flex min-w-0 flex-1 items-center gap-4">
      <div
        class="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
        role="progressbar"
        :aria-label="$t('progress.aria')"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="percent"
      >
        <div
          class="h-full rounded-full bg-blue-500 transition-[width] duration-300"
          :style="{ width: `${percent}%` }"
        />
      </div>
      <div class="flex min-w-0 max-w-[55%] shrink items-center gap-1.5 text-zinc-700 dark:text-zinc-300">
        <span class="shrink-0 tabular-nums">{{ summary }}</span>
        <span class="min-w-0 truncate" :title="currentFile">{{ currentFile }}</span>
      </div>
    </div>

    <div class="ml-auto flex shrink-0 items-center gap-4 text-zinc-400 dark:text-zinc-500">
      <span v-if="showPc" class="flex items-center gap-1.5" :title="pcLabel">
        <svg
          class="h-3.5 w-3.5 shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M22 12H2" />
          <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
          <path d="M6 16h.01" />
        </svg>
        <span class="tabular-nums">{{ pcLabel }}</span>
      </span>
      <span v-if="showPhone" class="flex items-center gap-1.5" :title="phoneLabel">
        <svg
          class="h-3.5 w-3.5 shrink-0"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="7" y="2" width="10" height="20" rx="2" />
          <path d="M11 18h2" />
        </svg>
        <span class="tabular-nums">{{ phoneLabel }}</span>
      </span>
    </div>
  </footer>
</template>
