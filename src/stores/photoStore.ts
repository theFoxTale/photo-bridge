import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import type { Album } from '../types/album'
import type { Photo } from '../types/photo'

type WailsApp = NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']>

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

export const usePhotoStore = defineStore('photo', () => {
  const albums = ref<Album[]>([])
  const photos = ref<Photo[]>([])
  const selectedIds = ref<Set<string>>(new Set())
  const activeAlbumId = ref<string | null>(null)
  const error = ref<string | null>(null)

  // Reported by the Go backend (build tag). `mock` means sample data.
  const libraryMode = ref<'device' | 'mock' | null>(null)
  const isDemoMode = computed(() => libraryMode.value === 'mock')
  // True after the active device vanished, until another device is loaded.
  const disconnected = ref(false)

  const thumbnailCache = reactive(new Map<string, string>())
  const failedThumbnails = reactive(new Set<string>())

  // Number of Go calls for albums/photos that have not settled yet.
  const pending = ref(0)
  const isLoading = computed(() => pending.value > 0)

  const selectedCount = computed(() => selectedIds.value.size)

  // Device the loaded data belongs to; thumbnails are requested for it.
  let currentDeviceId: string | null = null
  // Incremented to invalidate in-flight responses that were superseded.
  let albumsRun = 0
  let photosRun = 0
  const pendingThumbnails = new Map<string, Promise<void>>()

  function clearSelection(): void {
    if (selectedIds.value.size > 0) {
      selectedIds.value = new Set()
    }
  }

  function resetForDevice(deviceId: string | null): void {
    currentDeviceId = deviceId
    albumsRun++
    photosRun++
    albums.value = []
    photos.value = []
    activeAlbumId.value = null
    error.value = null
    selectedIds.value = new Set()
    thumbnailCache.clear()
    failedThumbnails.clear()
    pendingThumbnails.clear()
  }

  /** Drops everything, for example when no device is active any more. */
  function reset(): void {
    const hadDevice = currentDeviceId !== null
    resetForDevice(null)
    disconnected.value = hadDevice
  }

  async function loadLibraryMode(): Promise<void> {
    if (libraryMode.value !== null) return
    try {
      libraryMode.value = await getApi().LibraryMode()
    } catch {
      // Unknown mode: treat as a real device, the banner stays hidden.
      libraryMode.value = 'device'
    }
  }

  async function loadAlbums(deviceId: string): Promise<void> {
    if (deviceId.trim() === '') {
      error.value = 'Cannot load albums without a device.'
      return
    }
    if (deviceId !== currentDeviceId) {
      resetForDevice(deviceId)
    }
    disconnected.value = false
    void loadLibraryMode()

    const run = ++albumsRun
    pending.value++
    error.value = null
    try {
      const result = await getApi().FetchAlbums(deviceId)
      if (run !== albumsRun) return
      albums.value = result
    } catch (err) {
      if (run !== albumsRun) return
      albums.value = []
      error.value = toMessage(err)
    } finally {
      pending.value--
    }
  }

  async function loadPhotos(deviceId: string, albumId: string): Promise<void> {
    if (deviceId.trim() === '' || albumId.trim() === '') {
      error.value = 'Cannot load photos without a device and an album.'
      return
    }
    if (deviceId !== currentDeviceId) {
      resetForDevice(deviceId)
    }

    const run = ++photosRun
    activeAlbumId.value = albumId
    photos.value = []
    clearSelection()
    pending.value++
    error.value = null
    try {
      const result = await getApi().FetchPhotos(deviceId, albumId)
      if (run !== photosRun) return
      photos.value = result
    } catch (err) {
      if (run !== photosRun) return
      photos.value = []
      error.value = toMessage(err)
    } finally {
      pending.value--
    }
  }

  /** Fetches a thumbnail once; later calls reuse the cache or the in-flight request. */
  function loadThumbnail(photoId: string): Promise<void> {
    const deviceId = currentDeviceId
    if (deviceId === null || photoId.trim() === '') return Promise.resolve()
    if (thumbnailCache.has(photoId)) return Promise.resolve()

    const inFlight = pendingThumbnails.get(photoId)
    if (inFlight) return inFlight

    const request = (async () => {
      try {
        const url = await getApi().FetchThumbnail(deviceId, photoId)
        if (deviceId !== currentDeviceId) return
        thumbnailCache.set(photoId, url)
        failedThumbnails.delete(photoId)
      } catch {
        if (deviceId !== currentDeviceId) return
        failedThumbnails.add(photoId)
      } finally {
        pendingThumbnails.delete(photoId)
      }
    })()

    pendingThumbnails.set(photoId, request)
    return request
  }

  function hasPhoto(id: string): boolean {
    return photos.value.some((photo) => photo.id === id)
  }

  /** Adds the photo to the selection, or removes it when already selected. */
  function toggleSelection(id: string): void {
    if (!hasPhoto(id)) return
    const next = new Set(selectedIds.value)
    if (next.has(id)) {
      next.delete(id)
    } else {
      next.add(id)
    }
    selectedIds.value = next
  }

  /** Makes the photo the only selected one. */
  function selectOnly(id: string): void {
    if (!hasPhoto(id)) return
    selectedIds.value = new Set([id])
  }

  function selectAll(): void {
    selectedIds.value = new Set(photos.value.map((photo) => photo.id))
  }

  return {
    albums,
    photos,
    selectedIds,
    activeAlbumId,
    isLoading,
    error,
    libraryMode,
    isDemoMode,
    disconnected,
    thumbnailCache,
    failedThumbnails,
    selectedCount,
    loadAlbums,
    loadPhotos,
    loadThumbnail,
    toggleSelection,
    selectOnly,
    selectAll,
    clearSelection,
    reset,
  }
})
