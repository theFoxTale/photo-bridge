<script setup lang="ts">
type ConnectionStatus = 'disconnected' | 'searching' | 'connected'

defineProps<{
  status: ConnectionStatus
  deviceName: string
  selectedAlbum: string
}>()

const emit = defineEmits<{
  selectAlbum: [album: string]
}>()

const albums: readonly string[] = ['Camera Roll', 'Favorites', 'Screenshots', 'Videos']
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
      <!-- Loading -->
      <div v-if="status === 'searching'" class="space-y-2" aria-busy="true" aria-live="polite">
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

      <!-- Empty -->
      <div
        v-else-if="status === 'disconnected'"
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
        <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">No device connected</p>
        <p class="mt-1 text-xs text-zinc-500 dark:text-zinc-400">
          Connect your phone to browse its albums.
        </p>
      </div>

      <!-- Connected: album tree -->
      <nav v-else aria-label="Albums">
        <div
          class="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm font-medium text-zinc-900 dark:text-zinc-100"
        >
          <svg
            class="h-4 w-4 text-zinc-500 dark:text-zinc-400"
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
          <span class="truncate">{{ deviceName }}</span>
        </div>

        <ul class="ml-4 mt-1 space-y-0.5 border-l border-zinc-200 pl-2 dark:border-zinc-800">
          <li v-for="album in albums" :key="album">
            <button
              type="button"
              class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              :class="
                album === selectedAlbum
                  ? 'bg-blue-500/10 font-medium text-blue-700 dark:text-blue-300'
                  : 'text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800'
              "
              :aria-current="album === selectedAlbum ? 'true' : undefined"
              @click="emit('selectAlbum', album)"
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
              <span class="truncate">{{ album }}</span>
            </button>
          </li>
        </ul>
      </nav>
    </div>
  </aside>
</template>
