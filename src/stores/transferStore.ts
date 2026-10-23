import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import type {
  TransferFormatOptions,
  TransferProgress,
  TransferResult,
} from '../types/transfer'
import { formatBytes, formatDuration } from '../utils/format'
import { useComparisonStore } from './comparisonStore'
import { useDeviceStore } from './deviceStore'
import { usePhotoStore } from './photoStore'
import { useToastStore } from './toastStore'

type WailsApp = NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']>

const PROGRESS_EVENT = 'transfer:progress'

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

function isTransferProgress(value: unknown): value is TransferProgress {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Record<string, unknown>
  return (
    typeof candidate.completed === 'number' &&
    typeof candidate.total === 'number' &&
    typeof candidate.bytesCopied === 'number' &&
    typeof candidate.bytesPerSec === 'number' &&
    typeof candidate.currentFile === 'string'
  )
}

export const useTransferStore = defineStore('transfer', () => {
  const isRunning = ref(false)
  /** True between pressing Cancel and the backend acknowledging it. */
  const isCancelling = ref(false)
  const isChoosingFolder = ref(false)
  const progress = ref<TransferProgress | null>(null)
  const result = ref<TransferResult | null>(null)
  const error = ref<string | null>(null)
  const destPath = ref('')
  const options = ref<TransferFormatOptions>({
    saveOriginal: true,
    saveJpg: false,
    jpgQuality: 95,
  })

  const deviceStore = useDeviceStore()
  const photoStore = usePhotoStore()
  const toastStore = useToastStore()

  // The store lives as long as the app, so the subscription is never removed.
  // `window.runtime` is missing outside the Wails shell; then no progress arrives.
  window.runtime?.EventsOn(PROGRESS_EVENT, (data: unknown) => {
    // A late event must not resurrect the progress of a finished transfer.
    if (isRunning.value && isTransferProgress(data)) {
      progress.value = data
    }
  })

  function photoName(photoId: string): string {
    const known = photoStore.photos.find((photo) => photo.id === photoId)
    if (known) return known.filename
    return photoId.split('/').pop() ?? photoId
  }

  /** Shows the summary toast, for example "42 transferred · 1 failed · 128.4 MB in 12s". */
  function announce(outcome: TransferResult): void {
    const parts = [
      `${outcome.succeeded.length} transferred`,
      `${outcome.failed.length} failed`,
      `${formatBytes(outcome.bytesCopied)} in ${formatDuration(outcome.durationMs)}`,
    ]
    const message = outcome.cancelled
      ? `Cancelled · ${parts.join(' · ')}`
      : parts.join(' · ')

    let detail: string | undefined
    const first = outcome.failed[0]
    if (first) {
      const more = outcome.failed.length - 1
      detail = `${photoName(first.photoId)}: ${first.reason}`
      if (more > 0) detail += ` (and ${more} more)`
    }

    const nothingWorked = outcome.succeeded.length === 0 && outcome.failed.length > 0
    const kind = nothingWorked
      ? 'error'
      : outcome.failed.length > 0 || outcome.cancelled
        ? 'warning'
        : 'success'
    toastStore.show({ kind, message, detail }, detail ? 14000 : 8000)
  }

  /**
   * Compares the connected device with the destination folder without blocking
   * the grid. The comparison store keeps the previous result visible until the
   * new one arrives.
   */
  function runBackgroundCompare(): Promise<void> {
    const folder = destPath.value.trim()
    if (folder === '' || deviceStore.activeDeviceId === null) return Promise.resolve()
    return useComparisonStore().runComparison(folder)
  }

  /** Opens the native folder picker. Keeps the current folder when cancelled. */
  async function chooseDestFolder(): Promise<void> {
    if (isChoosingFolder.value || isRunning.value) return
    isChoosingFolder.value = true
    try {
      const chosen = await getApi().SelectFolder()
      if (chosen !== '') {
        const sameFolder = chosen === destPath.value
        destPath.value = chosen
        error.value = null
        // Setting the same path again does not notify the watcher below.
        if (sameFolder) void runBackgroundCompare()
      }
    } catch (err) {
      error.value = toMessage(err)
    } finally {
      isChoosingFolder.value = false
    }
  }

  /**
   * Transfers the given photos of the active device to the chosen folder.
   * `overwritePaths` maps photo IDs to existing local files their originals
   * replace; all other photos never overwrite anything.
   */
  async function startTransfer(
    photoIds: string[],
    overwritePaths?: Record<string, string>,
  ): Promise<void> {
    if (isRunning.value) return
    error.value = null
    result.value = null

    if (photoIds.length === 0) {
      error.value = 'Select at least one photo to transfer.'
      return
    }
    const deviceId = deviceStore.activeDeviceId
    if (deviceId === null) {
      error.value = 'No device is connected.'
      return
    }
    if (destPath.value.trim() === '') {
      error.value = 'Choose a destination folder first.'
      return
    }

    isRunning.value = true
    isCancelling.value = false
    progress.value = {
      completed: 0,
      total: photoIds.length,
      bytesCopied: 0,
      bytesPerSec: 0,
      currentFile: '',
    }
    try {
      const outcome = await getApi().TransferPhotos(deviceId, photoIds, {
        ...options.value,
        destPath: destPath.value,
        ...(overwritePaths ? { overwritePaths } : {}),
      })
      result.value = outcome
      announce(outcome)
      if (outcome.succeeded.length > 0) {
        void runBackgroundCompare()
      }
    } catch (err) {
      error.value = toMessage(err)
    } finally {
      isRunning.value = false
      isCancelling.value = false
      progress.value = null
    }
  }

  // A newly chosen folder (or a phone that connects after the folder was
  // chosen) is scanned in the background. A transfer that is still running
  // rescans itself once it finishes successfully.
  watch(
    [destPath, () => deviceStore.activeDeviceId],
    ([path, deviceId]) => {
      if (path.trim() === '' || deviceId === null || isRunning.value) return
      void runBackgroundCompare()
    },
  )

  /** Asks the backend to stop; the running `startTransfer` then settles with a partial result. */
  async function cancelTransfer(): Promise<void> {
    if (!isRunning.value || isCancelling.value) return
    isCancelling.value = true
    try {
      await getApi().CancelTransfer()
    } catch (err) {
      isCancelling.value = false
      error.value = toMessage(err)
    }
  }

  return {
    isRunning,
    isCancelling,
    isChoosingFolder,
    progress,
    result,
    error,
    destPath,
    options,
    startTransfer,
    cancelTransfer,
    chooseDestFolder,
    runBackgroundCompare,
  }
})
