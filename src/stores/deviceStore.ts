import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Device } from '../types/device'

export type DeviceStatus = 'idle' | 'searching' | 'connected' | 'error'

function getApi(): NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']> {
  const api = window.go?.main?.App
  if (!api) {
    throw new Error('Wails runtime is not available. Run the app with `wails dev` or `wails build`.')
  }
  return api
}

function toMessage(err: unknown): string {
  if (typeof err === 'string' && err !== '') return err
  if (err instanceof Error && err.message !== '') return err.message
  return 'Unknown error'
}

export const useDeviceStore = defineStore('device', () => {
  const devices = ref<Device[]>([])
  const status = ref<DeviceStatus>('idle')
  const activeDeviceId = ref<string | null>(null)
  const error = ref<string | null>(null)

  // Incremented whenever a run is started or cancelled, so a late response
  // from a superseded `DiscoverDevices` call can be recognised and ignored.
  let runId = 0

  const activeDevice = computed<Device | null>(
    () => devices.value.find((device) => device.id === activeDeviceId.value) ?? null,
  )

  async function startDiscovery(): Promise<void> {
    if (status.value === 'searching') return

    const currentRun = ++runId
    status.value = 'searching'
    error.value = null

    try {
      const found = await getApi().DiscoverDevices()
      if (currentRun !== runId) return

      devices.value = found
      if (!found.some((device) => device.id === activeDeviceId.value)) {
        activeDeviceId.value = found[0]?.id ?? null
      }
      status.value = found.length > 0 ? 'connected' : 'idle'
    } catch (err) {
      if (currentRun !== runId) return
      error.value = toMessage(err)
      status.value = 'error'
    }
  }

  async function stopDiscovery(): Promise<void> {
    if (status.value !== 'searching') return

    // Invalidate the in-flight run first so its result is dropped.
    runId++
    status.value = devices.value.length > 0 ? 'connected' : 'idle'

    try {
      await getApi().StopDiscovery()
    } catch (err) {
      error.value = toMessage(err)
      status.value = 'error'
    }
  }

  function selectDevice(id: string): void {
    if (!devices.value.some((device) => device.id === id)) {
      error.value = `Unknown device: ${id}`
      return
    }
    activeDeviceId.value = id
    error.value = null
  }

  return {
    devices,
    status,
    activeDeviceId,
    error,
    activeDevice,
    startDiscovery,
    stopDiscovery,
    selectDevice,
  }
})
