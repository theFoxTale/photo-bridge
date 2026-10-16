<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import BottomBar from './components/layout/BottomBar.vue'
import LeftSidebar from './components/layout/LeftSidebar.vue'
import RightSidebar from './components/layout/RightSidebar.vue'
import TopBar from './components/layout/TopBar.vue'
import PhotoGrid from './components/photos/PhotoGrid.vue'
import { useDeviceDiscovery } from './composables/useDeviceDiscovery'

type ConnectionStatus = 'disconnected' | 'searching' | 'connected'

interface TransferProgress {
  done: number
  total: number
  speedMBps: number
  etaSeconds: number
}

const { status: deviceStatus, devices, refresh } = useDeviceDiscovery()

// The layout components use a three-state view of the device store's status.
// A refresh while devices are already known keeps showing them as connected.
const status = computed<ConnectionStatus>(() => {
  switch (deviceStatus.value) {
    case 'connected':
      return 'connected'
    case 'searching':
      return devices.value.length > 0 ? 'connected' : 'searching'
    default:
      return 'disconnected'
  }
})

const isDark = ref(true)
const selectedAlbum = ref('Camera Roll')
const selectedIds = ref<string[]>([])
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

// Selection and any running preview are meaningless once the device is gone.
watch(status, (value) => {
  if (value !== 'connected') {
    selectedIds.value = []
    stopProgress()
  }
})

function toggleTheme(): void {
  isDark.value = !isDark.value
}

function toggleTile(id: string): void {
  selectedIds.value = selectedIds.value.includes(id)
    ? selectedIds.value.filter((existing) => existing !== id)
    : [...selectedIds.value, id]
}

// UI-only preview of the bottom bar's progress state; no files are transferred yet.
function startTransfer(): void {
  if (selectedIds.value.length === 0 || progressTimer !== null) return
  const total = selectedIds.value.length
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
      <LeftSidebar
        :selected-album="selectedAlbum"
        @select-album="selectedAlbum = $event"
      />
      <PhotoGrid
        :status="status"
        :album-name="selectedAlbum"
        :selected-ids="selectedIds"
        @toggle="toggleTile"
      />
      <RightSidebar
        :selected-count="selectedIds.length"
        :busy="progress !== null"
        @transfer="startTransfer"
      />
    </main>

    <BottomBar :progress="progress" />
  </div>
</template>
