<script setup lang="ts">
import { computed } from 'vue'
import { useDeviceDiscovery } from '../../composables/useDeviceDiscovery'
import { usePhotoLibrary } from '../../composables/usePhotoLibrary'
import type { Photo } from '../../types/photo'
import PhotoTile from './PhotoTile.vue'

const { activeDeviceId, status } = useDeviceDiscovery()
const {
  store,
  albums,
  photos,
  selectedIds,
  selectedCount,
  activeAlbumId,
  isLoading,
  error,
  isDemoMode,
  disconnected,
  reload,
  selectAlbum,
  toggleSelection,
  selectOnly,
  selectAll,
  clearSelection,
  loadThumbnail,
} = usePhotoLibrary()

const albumName = computed(
  () => albums.value.find((album) => album.id === activeAlbumId.value)?.name ?? 'Photos',
)

const isSearchingDevice = computed(
  () => activeDeviceId.value === null && status.value === 'searching',
)

// Retries loading the current album's photos; falls back to a full reload when
// the albums themselves failed to load.
function retry(): void {
  if (activeAlbumId.value !== null) {
    void selectAlbum(activeAlbumId.value)
  } else {
    void reload()
  }
}

function thumbUrlFor(photo: Photo): string | null {
  return store.thumbnailCache.get(photo.id) ?? (photo.thumbUrl !== '' ? photo.thumbUrl : null)
}

function onSelect(photoId: string, mode: 'only' | 'toggle'): void {
  if (mode === 'toggle') {
    toggleSelection(photoId)
  } else {
    selectOnly(photoId)
  }
}
</script>

<template>
  <section class="flex min-w-0 flex-1 flex-col bg-zinc-50 dark:bg-zinc-950" aria-label="Photos">
    <div
      class="flex h-12 shrink-0 items-center justify-between gap-3 border-b border-zinc-200 px-4 dark:border-zinc-800"
    >
      <h2 class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
        {{ activeDeviceId === null ? 'Photos' : albumName }}
      </h2>
      <div v-if="photos.length > 0" class="flex shrink-0 items-center gap-3">
        <span class="text-xs text-zinc-500 dark:text-zinc-400" aria-live="polite">
          {{ selectedCount }} of {{ photos.length }} selected
        </span>
        <button
          type="button"
          class="rounded px-2 py-1 text-xs font-medium text-blue-600 hover:bg-blue-500/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-40 dark:text-blue-400"
          :disabled="selectedCount === photos.length"
          @click="selectAll"
        >
          Select all
        </button>
        <button
          type="button"
          class="rounded px-2 py-1 text-xs font-medium text-zinc-600 hover:bg-zinc-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-40 dark:text-zinc-300 dark:hover:bg-zinc-800"
          :disabled="selectedCount === 0"
          @click="clearSelection"
        >
          Clear
        </button>
      </div>
    </div>

    <div
      v-if="isDemoMode"
      class="shrink-0 border-b border-amber-500/30 bg-amber-500/10 px-4 py-1.5 text-center text-xs font-medium text-amber-700 dark:text-amber-300"
      role="status"
    >
      Demo mode — showing sample photos
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-4">
      <!-- Loading -->
      <div
        v-if="isLoading || isSearchingDevice"
        class="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3"
        aria-busy="true"
      >
        <div
          v-for="n in 12"
          :key="n"
          class="aspect-square animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800"
        />
      </div>

      <!-- Device disconnected -->
      <div
        v-else-if="activeDeviceId === null && disconnected"
        class="flex h-full flex-col items-center justify-center text-center"
        role="status"
      >
        <svg
          class="mb-3 h-12 w-12 text-zinc-400 dark:text-zinc-600"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="7" y="2" width="10" height="20" rx="2" />
          <path d="m3 3 18 18" />
        </svg>
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">Device disconnected</p>
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          Reconnect your phone; it is picked up automatically.
        </p>
      </div>

      <!-- No device -->
      <div
        v-else-if="activeDeviceId === null"
        class="flex h-full flex-col items-center justify-center text-center"
      >
        <svg
          class="mb-3 h-12 w-12 text-zinc-400 dark:text-zinc-600"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="3" y="5" width="18" height="14" rx="2" />
          <circle cx="9" cy="10" r="1.5" />
          <path d="m21 16-5-5-9 8" />
        </svg>
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">No photos to show</p>
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          Connect a device to see its photos here.
        </p>
      </div>

      <!-- Error -->
      <div
        v-else-if="error"
        class="flex h-full flex-col items-center justify-center px-6 text-center"
        role="alert"
      >
        <svg
          class="mb-3 h-12 w-12 text-red-500"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="10" />
          <path d="M12 8v4M12 16h.01" />
        </svg>
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">Could not load photos</p>
        <p class="mt-1 max-w-md break-words text-xs text-zinc-500 dark:text-zinc-400">
          {{ error }}
        </p>
        <button
          type="button"
          class="mt-3 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
          @click="retry"
        >
          Retry
        </button>
      </div>

      <!-- Empty album -->
      <div
        v-else-if="photos.length === 0"
        class="flex h-full flex-col items-center justify-center text-center"
      >
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">This album is empty</p>
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          Pick another album from the sidebar.
        </p>
      </div>

      <!-- Grid -->
      <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3">
        <PhotoTile
          v-for="photo in photos"
          :key="photo.id"
          :photo="photo"
          :selected="selectedIds.has(photo.id)"
          :thumb-url="thumbUrlFor(photo)"
          :thumb-failed="store.failedThumbnails.has(photo.id)"
          :sync-status="store.syncStatusFor(photo.id)"
          @select="onSelect(photo.id, $event)"
          @visible="loadThumbnail(photo.id)"
        />
      </div>
    </div>
  </section>
</template>
