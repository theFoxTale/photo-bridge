import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { useDeviceStore } from './deviceStore'
import { useTransferStore } from './transferStore'

export const useSystemStore = defineStore('system', () => {
  /** Null until a read for the current destination succeeds. */
  const pcFree = ref<number | null>(null)
  /** Null until a read for the current device succeeds. Zero means the phone reported no free space. */
  const iPhoneFree = ref<number | null>(null)

  const deviceStore = useDeviceStore()
  const transferStore = useTransferStore()

  let request = 0

  async function fetchStorageInfo(deviceId: string | null, destPath: string): Promise<void> {
    const current = ++request
    const api = window.go?.main?.App
    if (!api?.GetStorageInfo) return
    try {
      const info = await api.GetStorageInfo(deviceId ?? '', destPath)
      if (current !== request) return
      if (typeof info.pcFree === 'number' && Number.isFinite(info.pcFree)) {
        pcFree.value = info.pcFree
      }
      if (typeof info.iPhoneFree === 'number' && Number.isFinite(info.iPhoneFree)) {
        iPhoneFree.value = info.iPhoneFree
      }
    } catch {
      // Keep the last reading. The first failure leaves the side blank.
    }
  }

  function refresh(): void {
    void fetchStorageInfo(deviceStore.activeDeviceId, transferStore.destPath)
  }

  watch(
    [() => deviceStore.activeDeviceId, () => transferStore.destPath],
    ([deviceId, destPath], previous) => {
      if (previous !== undefined && deviceId !== previous[0]) {
        iPhoneFree.value = null
      }
      if (previous !== undefined && destPath !== previous[1]) {
        pcFree.value = null
      }
      refresh()
    },
    { immediate: true },
  )

  watch(
    () => transferStore.isRunning,
    (running, previous) => {
      if (previous === true && running === false) refresh()
    },
  )

  return { pcFree, iPhoneFree, fetchStorageInfo }
})
