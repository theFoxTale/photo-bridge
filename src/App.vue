<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import BottomBar from './components/layout/BottomBar.vue'
import LeftSidebar from './components/layout/LeftSidebar.vue'
import RightSidebar from './components/layout/RightSidebar.vue'
import TopBar from './components/layout/TopBar.vue'
import PhotoGrid from './components/photos/PhotoGrid.vue'
import { useDeviceDiscovery } from './composables/useDeviceDiscovery'
import { usePhotoLibrary } from './composables/usePhotoLibrary'

interface TransferProgress {
  done: number
  total: number
  speedMBps: number
  etaSeconds: number
}

const { activeDeviceId, refresh } = useDeviceDiscovery()
// The only place that enables auto-sync: it loads the library for the active device.
const { selectedIds } = usePhotoLibrary({ autoSync: true })

const isDark = ref(true)
const progress = ref<TransferProgress | null>(null)

let progressTimer: ReturnType<typeof setInterval> | null = null

// Dark is the default theme; the `dark` class on <html> drives Tailwind's `dark:` variant.
watch(
  isDark,
  (value) => {
    document.documentElement.classList.toggle('dark', value)
  },
  { immediate: true },
)

function stopProgress(): void {
  if (progressTimer !== null) {
    clearInterval(progressTimer)
    progressTimer = null
  }
  progress.value = null
}

// A running preview is meaningless once the device is gone.
watch(activeDeviceId, (value) => {
  if (value === null) {
    stopProgress()
  }
})

function toggleTheme(): void {
  isDark.value = !isDark.value
}

// UI-only preview of the bottom bar's progress state; no files are transferred yet.
function startTransfer(): void {
  const total = selectedIds.value.size
  if (total === 0 || progressTimer !== null) return
  progress.value = { done: 0, total, speedMBps: 12.4, etaSeconds: total * 2 }
  progressTimer = setInterval(() => {
    const current = progress.value
    if (!current) {
      stopProgress()
      return
    }
    const done = current.done + 1
    if (done >= current.total) {
      stopProgress()
      return
    }
    progress.value = { ...current, done, etaSeconds: (current.total - done) * 2 }
  }, 2000)
}

onMounted(refresh)

onBeforeUnmount(stopProgress)
</script>

<template>
  <div
    class="flex h-screen flex-col bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100"
  >
    <TopBar :is-dark="isDark" @toggle-theme="toggleTheme" />

    <main class="flex min-h-0 flex-1">
      <LeftSidebar />
      <PhotoGrid />
      <RightSidebar :busy="progress !== null" @transfer="startTransfer" />
    </main>

    <BottomBar :progress="progress" />
  </div>
</template>
