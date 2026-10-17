import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { Device } from '../types/device'

export type DeviceStatus = 'idle' | 'searching' | 'connected' | 'error' | 'blocked_by_apple'

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
  // True while a silent (background) poll is in flight.
  let polling = false
  // The last status that was not 'searching'; restored when a search is stopped.
  let settledStatus: DeviceStatus = 'idle'

  const activeDevice = computed<Device | null>(
    () => devices.value.find((device) => device.id === activeDeviceId.value) ?? null,
  )

  /**
   * Searches for devices. A `silent` run is a background poll: it does not
   * switch the status to 'searching', so the UI does not flicker, and it is
   * skipped while another search is running.
   */
  async function startDiscovery(options: { silent?: boolean } = {}): Promise<void> {
    const silent = options.silent === true
    if (status.value === 'searching') return
    if (silent && polling) return

    const currentRun = ++runId
    if (silent) {
      polling = true
    } else {
      settledStatus = status.value
      status.value = 'searching'
      error.value = null
    }

    try {
      const result = await getApi().DiscoverDevices()
      if (currentRun !== runId) return

      const found = result.devices
      devices.value = found
      if (!found.some((device) => device.id === activeDeviceId.value)) {
        activeDeviceId.value = found[0]?.id ?? null
      }
      // Devices always win; otherwise the backend says whether the phone is
      // visible on the network but hidden from Apple Devices.
      if (found.length > 0) {
        status.value = 'connected'
      } else {
        status.value = result.status === 'blocked_by_apple' ? 'blocked_by_apple' : 'idle'
      }
    } catch (err) {
      if (currentRun !== runId) return
      // Without a working device list nothing can be trusted any more.
      devices.value = []
      activeDeviceId.value = null
      error.value = toMessage(err)
      status.value = 'error'
    } finally {
      if (silent) polling = false
    }
  }

  async function stopDiscovery(): Promise<void> {
    if (status.value !== 'searching') return

    // Invalidate the in-flight run first so its result is dropped.
    runId++
    if (devices.value.length > 0) {
      status.value = 'connected'
    } else {
      status.value = settledStatus === 'connected' ? 'idle' : settledStatus
    }

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
