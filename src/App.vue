<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'
import BottomBar from './components/layout/BottomBar.vue'
import LeftSidebar from './components/layout/LeftSidebar.vue'
import RightSidebar from './components/layout/RightSidebar.vue'
import TopBar from './components/layout/TopBar.vue'
import PhotoGrid from './components/photos/PhotoGrid.vue'
import ToastHost from './components/ui/ToastHost.vue'
import { useDeviceDiscovery } from './composables/useDeviceDiscovery'
import { usePhotoLibrary } from './composables/usePhotoLibrary'
import { useSettingsStore } from './stores/settingsStore'

const POLL_INTERVAL_MS = 15_000

const { store: deviceStore, refresh } = useDeviceDiscovery()
// The only place that enables auto-sync: it loads the library for the active device.
usePhotoLibrary({ autoSync: true })
// Applies the saved theme and language before the first paint of this tree.
useSettingsStore()

let pollTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  void refresh()
  // Background polling notices a phone being plugged in or removed.
  pollTimer = setInterval(() => {
    void deviceStore.startDiscovery({ silent: true })
  }, POLL_INTERVAL_MS)
})

onBeforeUnmount(() => {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<template>
  <div
    class="flex h-screen flex-col bg-zinc-50 text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100"
  >
    <TopBar />

    <main class="flex min-h-0 flex-1">
      <LeftSidebar />
      <PhotoGrid />
      <RightSidebar />
    </main>

    <BottomBar />
    <ToastHost />
  </div>
</template>
