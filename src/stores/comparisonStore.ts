import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type {
  CompareFilter,
  CompareProgress,
  CompareResult,
  ComparePhase,
  ComparisonRow,
  ComparisonStatus,
  ConflictResolution,
} from '../types/compare'
import { useDeviceStore } from './deviceStore'
import { usePhotoStore } from './photoStore'
import { useTransferStore } from './transferStore'

type WailsApp = NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']>

const PROGRESS_EVENT = 'compare:progress'
const PHASES: readonly ComparePhase[] = ['scanning', 'listing', 'hashing', 'done']

const FILTER_STATUS: Record<Exclude<CompareFilter, 'all'>, ComparisonStatus> = {
  new: 'new',
  existing: 'existing',
  conflicts: 'conflict',
  missing: 'missing',
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
  /** Whether the comparison view is showing. */
  const isOpen = ref(false)
  const isRunning = ref(false)
  /** True between pressing Cancel and the backend acknowledging it. */
  const isCancelling = ref(false)
  /** The last run was stopped by the user, so there is no result. */
  const wasCancelled = ref(false)
  /** Files were transferred after the comparison, so the result is outdated. */
  const isStale = ref(false)
  const isChoosingFolder = ref(false)
  const result = ref<CompareResult | null>(null)
  const filter = ref<CompareFilter>('all')
  const localPath = ref('')
  /** The folder `result` was computed for. */
  const comparedPath = ref('')
  const progress = ref<CompareProgress | null>(null)
  const error = ref<string | null>(null)
  const conflictResolution = ref<ConflictResolution>('skip')
  const selectedKeys = ref<Set<string>>(new Set())

  const deviceStore = useDeviceStore()
  const photoStore = usePhotoStore()
  const transferStore = useTransferStore()

  // The store lives as long as the app, so the subscription is never removed.
  // `window.runtime` is missing outside the Wails shell; then no progress arrives.
  window.runtime?.EventsOn(PROGRESS_EVENT, (data: unknown) => {
    if (isRunning.value && isCompareProgress(data)) {
      progress.value = data
    }
  })

  const rows = computed<ComparisonRow[]>(() => {
    const current = result.value
    if (!current) return []
    return [
      ...current.newOnDevice.map<ComparisonRow>((photo) => ({
        key: `device:${photo.id}`,
        status: 'new',
        filename: photo.filename,
        date: photo.date,
        size: photo.size,
        photo,
        localPath: null,
        reason: null,
      })),
      ...current.conflicts.map<ComparisonRow>((conflict) => ({
        key: `device:${conflict.photo.id}`,
        status: 'conflict',
        filename: conflict.photo.filename,
        date: conflict.photo.date,
        size: conflict.photo.size,
        photo: conflict.photo,
        localPath: conflict.localPath,
        reason: conflict.reason,
      })),
      ...current.alreadyOnPc.map<ComparisonRow>((photo) => ({
        key: `device:${photo.id}`,
        status: 'existing',
        filename: photo.filename,
        date: photo.date,
        size: photo.size,
        photo,
        localPath: null,
        reason: null,
      })),
      ...current.missingOnPc.map<ComparisonRow>((file) => ({
        key: `local:${file.path}`,
        status: 'missing',
        filename: file.filename,
        date: file.modTime,
        size: file.size,
        photo: null,
        localPath: file.path,
        reason: null,
      })),
    ]
  })

  const counts = computed<Record<CompareFilter, number>>(() => {
    const current = result.value
    return {
      all: rows.value.length,
      new: current?.newOnDevice.length ?? 0,
      existing: current?.alreadyOnPc.length ?? 0,
      conflicts: current?.conflicts.length ?? 0,
      missing: current?.missingOnPc.length ?? 0,
    }
  })

  const visibleRows = computed<ComparisonRow[]>(() => {
    if (filter.value === 'all') return rows.value
    const wanted = FILTER_STATUS[filter.value]
    return rows.value.filter((row) => row.status === wanted)
  })

  const selectedRows = computed<ComparisonRow[]>(() =>
    rows.value.filter((row) => selectedKeys.value.has(row.key)),
  )

  function open(): void {
    if (localPath.value.trim() === '') {
      localPath.value = transferStore.destPath
    }
    isOpen.value = true
  }

  function close(): void {
    isOpen.value = false
  }

  function setFilter(next: CompareFilter): void {
    filter.value = next
  }

  function clearSelection(): void {
    if (selectedKeys.value.size > 0) selectedKeys.value = new Set()
  }

  function toggleRow(key: string): void {
    const next = new Set(selectedKeys.value)
    if (next.has(key)) {
      next.delete(key)
    } else {
      next.add(key)
    }
    selectedKeys.value = next
  }

  /** Selects or deselects every row of the current filter. */
  function setVisibleSelected(selected: boolean): void {
    const next = new Set(selectedKeys.value)
    for (const row of visibleRows.value) {
      if (selected) {
        next.add(row.key)
      } else {
        next.delete(row.key)
      }
    }
    selectedKeys.value = next
  }

  /**
   * Picks the folder to compare with. The transfer destination follows, so
   * transfers started from the comparison land in the folder that was compared.
   */
  async function chooseLocalFolder(): Promise<void> {
    if (isChoosingFolder.value || isRunning.value) return
    isChoosingFolder.value = true
    try {
      const chosen = await getApi().SelectFolder()
      if (chosen !== '') {
        localPath.value = chosen
        transferStore.destPath = chosen
        error.value = null
      }
    } catch (err) {
      error.value = toMessage(err)
    } finally {
      isChoosingFolder.value = false
    }
  }

  /**
   * Compares the active device with `path` (or the current `localPath`) and
   * opens the comparison view. Does nothing while a comparison is running.
   */
  async function runComparison(path?: string): Promise<void> {
    if (isRunning.value) return
    isOpen.value = true
    if (path !== undefined) localPath.value = path
    error.value = null
    wasCancelled.value = false

    const folder = localPath.value.trim()
    if (folder === '') {
      error.value = 'Choose a folder to compare with.'
      return
    }
    const deviceId = deviceStore.activeDeviceId
    if (deviceId === null) {
      error.value = 'No device is connected.'
      return
    }

    result.value = null
    isStale.value = false
    clearSelection()
    isRunning.value = true
    isCancelling.value = false
    progress.value = { scanned: 0, total: 0, phase: 'scanning' }
    try {
      const outcome = await getApi().CompareFolder(deviceId, folder)
      // The device may have been swapped while the comparison ran.
      if (deviceStore.activeDeviceId !== deviceId) return
      result.value = outcome
      comparedPath.value = folder
    } catch (err) {
      if (isCancelling.value) {
        wasCancelled.value = true
      } else {
        error.value = toMessage(err)
      }
    } finally {
      isRunning.value = false
      isCancelling.value = false
      progress.value = null
    }
  }

  /** Asks the backend to stop; the running `runComparison` then settles as cancelled. */
  async function cancelComparison(): Promise<void> {
    if (!isRunning.value || isCancelling.value) return
    isCancelling.value = true
    try {
      await getApi().CancelCompare()
    } catch (err) {
      isCancelling.value = false
      error.value = toMessage(err)
    }
  }

  /**
   * Transfers the device photos among `chosen` to the compared folder. Photos
   * in conflict follow `conflictResolution`: skipped, replacing the local file,
   * or saved next to it.
   */
  async function transferRows(chosen: ComparisonRow[]): Promise<void> {
    const ids: string[] = []
    const overwrite: Record<string, string> = {}
    for (const row of chosen) {
      if (!row.photo) continue
      if (row.status === 'conflict') {
        if (conflictResolution.value === 'skip') continue
        if (conflictResolution.value === 'overwrite' && row.localPath) {
          overwrite[row.photo.id] = row.localPath
        }
      }
      ids.push(row.photo.id)
    }
    if (ids.length === 0 || comparedPath.value === '') return

    transferStore.destPath = comparedPath.value
    photoStore.selectIds(ids)
    await transferStore.startTransfer(ids, Object.keys(overwrite).length > 0 ? overwrite : undefined)
  }

  /** Transfers every photo that is new on the device. */
  function transferNew(): Promise<void> {
    return transferRows(rows.value.filter((row) => row.status === 'new'))
  }

  /** Transfers new photos plus the conflicts, resolved as `conflictResolution` says. */
  function transferNewAndConflicts(): Promise<void> {
    return transferRows(rows.value.filter((row) => row.status === 'new' || row.status === 'conflict'))
  }

  /** Transfers the rows ticked in the table. */
  function transferSelected(): Promise<void> {
    return transferRows(selectedRows.value)
  }

  // A result belongs to one device.
  watch(
    () => deviceStore.activeDeviceId,
    () => {
      result.value = null
      comparedPath.value = ''
      isStale.value = false
      wasCancelled.value = false
      clearSelection()
    },
  )

  // After files were copied the result no longer describes the folder.
  watch(
    () => transferStore.isRunning,
    (running, wasRunning) => {
      if (wasRunning && !running && result.value && (transferStore.result?.succeeded.length ?? 0) > 0) {
        isStale.value = true
      }
    },
  )

  return {
    isOpen,
    isRunning,
    isCancelling,
    isChoosingFolder,
    wasCancelled,
    isStale,
    result,
    filter,
    localPath,
    comparedPath,
    progress,
    error,
    conflictResolution,
    selectedKeys,
    rows,
    counts,
    visibleRows,
    selectedRows,
    open,
    close,
    setFilter,
    toggleRow,
    setVisibleSelected,
    clearSelection,
    chooseLocalFolder,
    runComparison,
    cancelComparison,
    transferNew,
    transferNewAndConflicts,
    transferSelected,
  }
})
