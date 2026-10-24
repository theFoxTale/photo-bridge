<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDeviceDiscovery } from '../../composables/useDeviceDiscovery'
import { usePhotoLibrary } from '../../composables/usePhotoLibrary'
import type { Album } from '../../types/album'

const DEVICE_FOLDERS_KEY = 'photobridge.deviceFoldersExpanded'
const CAMERA_ROLL_ID = 'camera-roll'
const DEVICE_FOLDER_PREFIX = 'roll-'

const { devices, status, activeDeviceId, activeDevice, error, refresh, selectDevice } =
  useDeviceDiscovery()
const {
  albums,
  activeAlbumId,
  isLoading: isLoadingLibrary,
  selectAlbum,
} = usePhotoLibrary()

function readDeviceFoldersExpanded(): boolean {
  try {
    return localStorage.getItem(DEVICE_FOLDERS_KEY) === '1'
  } catch {
    return false
  }
}

const deviceFoldersExpanded = ref(readDeviceFoldersExpanded())

function toggleDeviceFolders(): void {
  deviceFoldersExpanded.value = !deviceFoldersExpanded.value
  try {
    localStorage.setItem(DEVICE_FOLDERS_KEY, deviceFoldersExpanded.value ? '1' : '0')
  } catch {
    // The section still opens for this session when storage is blocked.
  }
}

const cameraRoll = computed(
  () => albums.value.find((album) => album.id === CAMERA_ROLL_ID) ?? null,
)

const deviceFolders = computed(() =>
  albums.value.filter((album) => album.id.startsWith(DEVICE_FOLDER_PREFIX)),
)

const otherAlbums = computed(() =>
  albums.value.filter(
    (album) => album.id !== CAMERA_ROLL_ID && !album.id.startsWith(DEVICE_FOLDER_PREFIX),
  ),
)

type SidebarRow =
  | { kind: 'album'; id: string; album: Album; nested: boolean }
  | { kind: 'toggle'; id: 'device-folders' }

const sidebarRows = computed((): SidebarRow[] => {
  const rows: SidebarRow[] = []
  if (cameraRoll.value) {
    rows.push({ kind: 'album', id: cameraRoll.value.id, album: cameraRoll.value, nested: false })
  }
  if (deviceFolders.value.length > 0) {
    rows.push({ kind: 'toggle', id: 'device-folders' })
    if (deviceFoldersExpanded.value) {
      for (const album of deviceFolders.value) {
        rows.push({ kind: 'album', id: album.id, album, nested: true })
      }
    }
  }
  for (const album of otherAlbums.value) {
    rows.push({ kind: 'album', id: album.id, album, nested: false })
  }
  return rows
})

function albumClass(id: string): string {
  if (id === activeAlbumId.value) {
    return 'bg-blue-500/10 font-medium text-blue-700 dark:text-blue-300'
  }
  return 'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800'
}
</script>

<template>
  <aside
    class="flex w-[260px] shrink-0 flex-col border-r border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"
    aria-label="Device"
  >
    <div class="border-b border-zinc-200 px-4 py-3 dark:border-zinc-800">
      <h2 class="text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
        Device
      </h2>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-3">
      <!-- Loading (first search, nothing known yet) -->
      <div
        v-if="status === 'searching' && devices.length === 0"
        class="space-y-2"
        aria-busy="true"
        aria-live="polite"
      >
        <div class="h-8 animate-pulse rounded-md bg-zinc-200 dark:bg-zinc-800" />
        <div class="ml-4 space-y-2">
          <div
            v-for="n in 4"
            :key="n"
            class="h-7 animate-pulse rounded-md bg-zinc-200 dark:bg-zinc-800"
          />
        </div>
        <p class="pt-2 text-center text-xs text-zinc-500 dark:text-zinc-400">
          Searching for devices…
        </p>
      </div>

      <!-- Error -->
      <div
        v-else-if="status === 'error'"
        class="flex h-full flex-col items-center justify-center px-2 text-center"
        role="alert"
      >
        <svg
          class="mb-3 h-10 w-10 text-red-500"
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
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">Device search failed</p>
        <p class="mt-1 break-words text-xs text-zinc-500 dark:text-zinc-400">
          {{ error ?? 'Unknown error' }}
        </p>
        <button
          type="button"
          class="mt-3 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
          @click="refresh"
        >
          Try again
        </button>
      </div>

      <!-- Blocked: phone visible on Wi-Fi, but Apple Devices does not list it -->
      <div
        v-else-if="status === 'blocked_by_apple'"
        class="flex h-full flex-col items-center justify-center px-2 text-center"
        role="alert"
      >
        <svg
          class="mb-3 h-10 w-10 text-amber-500"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
          <path d="M12 9v4M12 17h.01" />
        </svg>
        <p class="text-sm font-medium text-amber-700 dark:text-amber-300">Apple Devices can't see your iPhone</p>
        <p class="mt-1 text-xs text-zinc-600 dark:text-zinc-400">
          iPhone found on Wi-Fi, but Apple Devices cannot see it. Please open Apple Devices on your
          PC, connect your iPhone via USB, and enable 'Show this iPhone when on Wi-Fi'.
        </p>
        <button
          type="button"
          class="mt-3 rounded-md bg-amber-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-amber-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-400"
          @click="refresh"
        >
          Retry
        </button>
      </div>

      <!-- Empty -->
      <div
        v-else-if="devices.length === 0"
        class="flex h-full flex-col items-center justify-center px-4 text-center"
      >
        <svg
          class="mb-3 h-10 w-10 text-zinc-400 dark:text-zinc-600"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="7" y="2" width="10" height="20" rx="2" />
          <path d="M11 18h2" />
        </svg>
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">No device found</p>
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          Make sure your phone is on the same Wi-Fi network.
        </p>
        <button
          type="button"
          class="mt-3 rounded-md border border-zinc-300 px-3 py-1.5 text-xs font-medium text-zinc-700 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          @click="refresh"
        >
          Search again
        </button>
      </div>

      <!-- Devices + album tree -->
      <div v-else class="space-y-4">
        <section aria-label="Devices">
          <p
            v-if="status === 'searching'"
            class="mb-1 px-2 text-xs text-zinc-500 dark:text-zinc-400"
            aria-live="polite"
          >
            Searching…
          </p>
          <ul class="space-y-0.5">
            <li v-for="device in devices" :key="device.id">
              <button
                type="button"
                class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                :class="
                  device.id === activeDeviceId
                    ? 'bg-blue-500/10 font-medium text-blue-700 dark:text-blue-300'
                    : 'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800'
                "
                :aria-current="device.id === activeDeviceId ? 'true' : undefined"
                :title="device.ip ? `${device.name} (${device.ip})` : device.name"
                @click="selectDevice(device.id)"
              >
                <svg
                  class="h-4 w-4 shrink-0"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <rect x="7" y="2" width="10" height="20" rx="2" />
                  <path d="M11 18h2" />
                </svg>
                <span class="min-w-0 flex-1">
                  <span class="block truncate">{{ device.name }}</span>
                  <span
                    v-if="device.ip"
                    class="block truncate text-xs font-normal text-zinc-500 dark:text-zinc-400"
                  >
                    {{ device.ip }}
                  </span>
                </span>
              </button>
            </li>
          </ul>
        </section>

        <nav
          v-if="activeDevice"
          class="border-t border-zinc-200 pt-3 dark:border-zinc-800"
          aria-label="Albums"
        >
          <p
            class="mb-1 truncate px-2 text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400"
          >
            Albums · {{ activeDevice.name }}
          </p>
          <div
            v-if="albums.length === 0 && isLoadingLibrary"
            class="ml-2 space-y-2 border-l border-zinc-200 pl-2 dark:border-zinc-800"
            aria-busy="true"
          >
            <div
              v-for="n in 3"
              :key="n"
              class="h-7 animate-pulse rounded-md bg-zinc-200 dark:bg-zinc-800"
            />
          </div>
          <p
            v-else-if="albums.length === 0"
            class="ml-2 border-l border-zinc-200 px-2 py-1.5 text-xs text-zinc-500 dark:border-zinc-800 dark:text-zinc-400"
          >
            No albums
          </p>
          <ul v-else class="ml-2 space-y-0.5 border-l border-zinc-200 pl-2 dark:border-zinc-800">
            <li
              v-for="row in sidebarRows"
              :key="row.id"
              :class="row.kind === 'album' && row.nested ? 'pl-3' : ''"
            >
              <button
                v-if="row.kind === 'toggle'"
                type="button"
                class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-xs font-medium text-zinc-500 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-400 dark:hover:bg-zinc-800"
                :aria-expanded="deviceFoldersExpanded"
                @click="toggleDeviceFolders"
              >
                <svg
                  class="h-3.5 w-3.5 shrink-0 transition-transform"
                  :class="deviceFoldersExpanded ? 'rotate-90' : ''"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="m9 18 6-6-6-6" />
                </svg>
                <span class="min-w-0 flex-1 truncate">Device Folders (Advanced)</span>
                <span class="shrink-0 tabular-nums">{{ deviceFolders.length }}</span>
              </button>
              <button
                v-else
                type="button"
                class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                :class="albumClass(row.album.id)"
                :aria-current="row.album.id === activeAlbumId ? 'true' : undefined"
                @click="selectAlbum(row.album.id)"
              >
                <svg
                  class="h-4 w-4 shrink-0"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  aria-hidden="true"
                >
                  <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
                </svg>
                <span class="min-w-0 flex-1 truncate">{{ row.album.name }}</span>
                <span class="shrink-0 text-xs tabular-nums text-zinc-500 dark:text-zinc-400">
                  {{ row.album.count }}
                </span>
              </button>
            </li>
          </ul>
        </nav>
      </div>
    </div>
  </aside>
</template>
