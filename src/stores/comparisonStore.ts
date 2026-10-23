import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { ComparePhase, CompareProgress, CompareResult } from '../types/compare'
import { useDeviceStore } from './deviceStore'

type WailsApp = NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']>

const PROGRESS_EVENT = 'compare:progress'
const PHASES: readonly ComparePhase[] = ['scanning', 'listing', 'hashing', 'done']

interface CompareRequest {
  id: number
  folder: string
  deviceId: string
}

function getApi(): WailsApp {
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

function isCompareProgress(value: unknown): value is CompareProgress {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Record<string, unknown>
  return (
    typeof candidate.scanned === 'number' &&
    typeof candidate.total === 'number' &&
    typeof candidate.phase === 'string' &&
    PHASES.includes(candidate.phase as ComparePhase)
  )
}

export const useComparisonStore = defineStore('comparison', () => {
  const isRunning = ref(false)
  const result = ref<CompareResult | null>(null)
  /** The folder `result` was computed for. */
  const comparedPath = ref('')
  const progress = ref<CompareProgress | null>(null)
  const error = ref<string | null>(null)

  const deviceStore = useDeviceStore()

  // The store lives as long as the app, so the subscription is never removed.
  // `window.runtime` is missing outside the Wails shell; then no progress arrives.
  window.runtime?.EventsOn(PROGRESS_EVENT, (data: unknown) => {
    if (isRunning.value && isCompareProgress(data)) {
      progress.value = data
    }
  })

  const counts = computed(() => {
    const current = result.value
    const fresh = current?.newOnDevice.length ?? 0
    const existing = current?.alreadyOnPc.length ?? 0
    const conflicts = current?.conflicts.length ?? 0
    return {
      totalOnDevice: fresh + existing + conflicts,
      new: fresh,
      existing,
      conflicts,
    }
  })

  // One scan at a time. A newer request cancels the current one and the loop
  // restarts with that request, so picking another folder does not wait out
  // the previous folder.
  let latest: CompareRequest | null = null
  let scanning = false
  let nextRequestId = 0

  /** False once a newer folder, a newer request, or a device change has taken over. */
  function stillCurrent(target: CompareRequest): boolean {
    return (
      latest !== null &&
      latest.id === target.id &&
      deviceStore.activeDeviceId === target.deviceId
    )
  }

  /**
   * Compares the active device's camera roll with `folder`. Safe to call
   * again while a comparison is running: the newer folder and device win.
   * A repeat scan of the same folder keeps the previous result until the new
   * one arrives, so the grid does not flicker.
   */
  async function runComparison(folder: string): Promise<void> {
    const deviceId = deviceStore.activeDeviceId
    const trimmed = folder.trim()
    if (deviceId === null || trimmed === '') {
      error.value = deviceId === null ? 'No device is connected.' : 'Choose a folder to compare with.'
      return
    }

    latest = { id: ++nextRequestId, folder: trimmed, deviceId }
    if (comparedPath.value !== '' && comparedPath.value !== trimmed) {
      result.value = null
      comparedPath.value = ''
    }
    error.value = null

    if (scanning) {
      try {
        await getApi().CancelCompare()
      } catch {
        // The scan already in progress notices the new request and restarts.
      }
      return
    }

    scanning = true
    let slotRetries = 0
    try {
      while (latest !== null && deviceStore.activeDeviceId === latest.deviceId) {
        const target = latest
        if (comparedPath.value !== '' && comparedPath.value !== target.folder) {
          result.value = null
          comparedPath.value = ''
        }

        isRunning.value = true
        progress.value = { scanned: 0, total: 0, phase: 'scanning' }
        try {
          const outcome = await getApi().CompareFolder(target.deviceId, target.folder)
          if (!stillCurrent(target)) continue
          result.value = outcome
          comparedPath.value = target.folder
          error.value = null
          break
        } catch (err) {
          if (!stillCurrent(target)) continue
          const message = toMessage(err)
          // The backend slot can still be held for a moment after a cancel.
          if (message.includes('already running') && slotRetries < 2) {
            slotRetries++
            try {
              await getApi().CancelCompare()
            } catch {
              // The following attempt reports the error if the slot stays taken.
            }
            continue
          }
          slotRetries = 0
          if (message.includes('comparison cancelled')) break
          error.value = message
          break
        }
      }
    } finally {
      scanning = false
      isRunning.value = false
      progress.value = null
    }
  }

  // A result belongs to one device. Drop it immediately so the grid does not
  // keep the previous phone's checkmarks. A scan that this same change already
  // started for the new device is left running.
  watch(
    () => deviceStore.activeDeviceId,
    (deviceId) => {
      const scanIsForThisDevice = latest !== null && latest.deviceId === deviceId
      if (!scanIsForThisDevice) {
        latest = null
        if (scanning) {
          void getApi().CancelCompare().catch(() => undefined)
        }
      }
      result.value = null
      comparedPath.value = ''
      error.value = null
    },
  )

  return {
    isRunning,
    result,
    comparedPath,
    progress,
    error,
    counts,
    runComparison,
  }
})
