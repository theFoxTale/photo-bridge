<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import type { Photo } from '../../types/photo'

const props = defineProps<{
  photo: Photo
  selected: boolean
  /** Data URL of the thumbnail, or null while it has not been loaded. */
  thumbUrl: string | null
  thumbFailed: boolean
}>()

const emit = defineEmits<{
  /** `only` replaces the selection, `toggle` adds or removes this photo. */
  select: [mode: 'only' | 'toggle']
  /** The tile scrolled into (or near) the viewport; load its thumbnail. */
  visible: []
}>()

const root = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

function stopObserving(): void {
  observer?.disconnect()
  observer = null
}

onMounted(() => {
  if (props.thumbUrl !== null) return
  const element = root.value
  if (!element) return

  if (typeof IntersectionObserver === 'undefined') {
    emit('visible')
    return
  }

  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        emit('visible')
        stopObserving()
      }
    },
    { rootMargin: '200px' },
  )
  observer.observe(element)
})

onBeforeUnmount(stopObserving)

function onTileClick(event: MouseEvent): void {
  emit('select', event.ctrlKey || event.metaKey ? 'toggle' : 'only')
}
</script>

<template>
  <div
    ref="root"
    class="group relative aspect-square w-full overflow-hidden rounded-lg ring-2 transition"
    :class="selected ? 'ring-blue-500' : 'ring-transparent hover:ring-zinc-400 dark:hover:ring-zinc-600'"
  >
    <button
      type="button"
      class="absolute inset-0 block h-full w-full focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-400"
      :aria-pressed="selected"
      :aria-label="`${selected ? 'Selected' : 'Select'} ${photo.filename}`"
      @click="onTileClick"
    >
      <img
        v-if="thumbUrl"
        :src="thumbUrl"
        :alt="photo.filename"
        draggable="false"
        class="h-full w-full select-none object-cover"
      />
      <span
        v-else-if="thumbFailed"
        class="flex h-full w-full flex-col items-center justify-center gap-1 bg-zinc-200 text-xs text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
      >
        <svg
          class="h-6 w-6"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="3" y="5" width="18" height="14" rx="2" />
          <path d="m4 4 16 16" />
        </svg>
        Preview unavailable
      </span>
      <span v-else class="block h-full w-full animate-pulse bg-zinc-200 dark:bg-zinc-800" />

      <!-- Hover overlay -->
      <span
        class="pointer-events-none absolute inset-x-0 bottom-0 flex items-end justify-between gap-2 bg-linear-to-t from-black/70 to-transparent px-2 pb-1.5 pt-6 text-[11px] text-white opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100"
      >
        <span class="truncate">{{ photo.filename }}</span>
        <span class="shrink-0 rounded bg-white/20 px-1 uppercase">{{ photo.format }}</span>
      </span>
    </button>

    <button
      type="button"
      role="checkbox"
      :aria-checked="selected"
      :aria-label="`${selected ? 'Deselect' : 'Select'} ${photo.filename}`"
      class="absolute right-2 top-2 flex h-5 w-5 items-center justify-center rounded-full border text-white shadow transition focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
      :class="
        selected
          ? 'border-blue-500 bg-blue-500 opacity-100'
          : 'border-white/80 bg-black/40 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100'
      "
      @click.stop="emit('select', 'toggle')"
    >
      <svg
        v-if="selected"
        class="h-3 w-3"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="3"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M20 6 9 17l-5-5" />
      </svg>
    </button>
  </div>
</template>
