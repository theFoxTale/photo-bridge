import { storeToRefs } from 'pinia'
import { watch } from 'vue'
import { usePhotoStore } from '../stores/photoStore'
import { useDeviceDiscovery } from './useDeviceDiscovery'

interface PhotoLibraryOptions {
  /**
   * When true, the library follows the active device: albums and the first
   * album's photos are loaded whenever it changes, and everything is cleared
   * when there is no active device. Enable this in exactly one place (App.vue).
   */
  autoSync?: boolean
}

/** Ties the photo store to the active device. */
export function usePhotoLibrary(options: PhotoLibraryOptions = {}) {
  const store = usePhotoStore()
  const { activeDeviceId } = useDeviceDiscovery()
  const { albums, photos, selectedIds, activeAlbumId, isLoading, error, selectedCount } =
    storeToRefs(store)

  /** Loads the albums of a device and then the photos of its first album. */
  async function loadLibrary(deviceId: string | null): Promise<void> {
    if (deviceId === null) {
      store.reset()
      return
    }
    await store.loadAlbums(deviceId)
    const first = store.albums[0]
    if (first && activeDeviceId.value === deviceId) {
      await store.loadPhotos(deviceId, first.id)
    }
  }

  /** Re-runs the full load for the active device (used by retry buttons). */
  function reload(): Promise<void> {
    return loadLibrary(activeDeviceId.value)
  }

  /** Loads the photos of an album on the active device. */
  async function selectAlbum(albumId: string): Promise<void> {
    const deviceId = activeDeviceId.value
    if (deviceId === null) return
    await store.loadPhotos(deviceId, albumId)
  }

  if (options.autoSync) {
    watch(activeDeviceId, (deviceId) => void loadLibrary(deviceId), { immediate: true })
  }

  return {
    store,
    albums,
    photos,
    selectedIds,
    selectedCount,
    activeAlbumId,
    isLoading,
    error,
    reload,
    selectAlbum,
    toggleSelection: store.toggleSelection,
    selectOnly: store.selectOnly,
    selectAll: store.selectAll,
    clearSelection: store.clearSelection,
    loadThumbnail: store.loadThumbnail,
  }
}
