import { storeToRefs } from 'pinia'
import { useDeviceStore } from '../stores/deviceStore'

/** Thin wrapper around the device store that adds a `refresh()` action. */
export function useDeviceDiscovery() {
  const store = useDeviceStore()
  const { devices, status, activeDeviceId, activeDevice, error } = storeToRefs(store)

  /** Cancels a running search (if any) and starts a fresh one. */
  async function refresh(): Promise<void> {
    if (status.value === 'searching') {
      await store.stopDiscovery()
    }
    await store.startDiscovery()
  }

  return {
    store,
    devices,
    status,
    activeDeviceId,
    activeDevice,
    error,
    refresh,
    stopDiscovery: store.stopDiscovery,
    selectDevice: store.selectDevice,
  }
}
