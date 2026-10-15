<script setup lang="ts">
import PhotoTile from './PhotoTile.vue'

type ConnectionStatus = 'disconnected' | 'searching' | 'connected'

defineProps<{
  status: ConnectionStatus
  albumName: string
  selectedIds: string[]
}>()

const emit = defineEmits<{
  toggle: [id: string]
}>()

const tiles: readonly { id: string; label: string }[] = Array.from({ length: 8 }, (_, i) => ({
  id: `tile-${i + 1}`,
  label: `Photo ${i + 1}`,
}))
</script>

<template>
  <section class="flex min-w-0 flex-1 flex-col bg-zinc-50 dark:bg-zinc-950" aria-label="Photos">
    <div
      class="flex h-12 shrink-0 items-center justify-between border-b border-zinc-200 px-4 dark:border-zinc-800"
    >
      <h2 class="truncate text-sm font-medium text-zinc-900 dark:text-zinc-100">
        {{ status === 'connected' ? albumName : 'Photos' }}
      </h2>
      <span
        v-if="status === 'connected'"
        class="text-xs text-zinc-500 dark:text-zinc-400"
        aria-live="polite"
      >
        {{ selectedIds.length }} of {{ tiles.length }} selected
      </span>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-4">
      <!-- Loading -->
      <div
        v-if="status === 'searching'"
        class="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3"
        aria-busy="true"
      >
        <div
          v-for="n in 8"
          :key="n"
          class="aspect-square animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800"
        />
      </div>

      <!-- Empty -->
      <div
        v-else-if="status === 'disconnected'"
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

      <!-- Grid -->
      <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-3">
        <PhotoTile
          v-for="tile in tiles"
          :key="tile.id"
          :label="tile.label"
          :selected="selectedIds.includes(tile.id)"
          @toggle="emit('toggle', tile.id)"
        />
      </div>
    </div>
  </section>
</template>
