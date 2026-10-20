<script setup lang="ts">
import { useVirtualList } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { computed, watch, watchEffect } from 'vue'
import { useComparisonStore } from '../../stores/comparisonStore'
import { usePhotoStore } from '../../stores/photoStore'
import type { CompareFilter, ComparisonStatus } from '../../types/compare'
import { formatBytes, formatDateTime } from '../../utils/format'

/** Fixed row height in px; the virtual list needs it up front. */
const ROW_HEIGHT = 56
/** From this many rows on only the visible ones are rendered. */
const VIRTUAL_THRESHOLD = 200

const comparisonStore = useComparisonStore()
const photoStore = usePhotoStore()
const { visibleRows, counts, filter, selectedKeys } = storeToRefs(comparisonStore)
const { thumbnailCache, failedThumbnails } = storeToRefs(photoStore)

const tabs: readonly { value: CompareFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'new', label: 'New on iPhone' },
  { value: 'existing', label: 'Already on PC' },
  { value: 'conflicts', label: 'Conflicts' },
  { value: 'missing', label: 'Missing on PC' },
]

const badges: Record<ComparisonStatus, { label: string; classes: string }> = {
  new: {
    label: 'New on iPhone',
    classes: 'bg-blue-500/15 text-blue-700 dark:text-blue-300',
  },
  existing: {
    label: 'Already on PC',
    classes: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
  },
  conflict: {
    label: 'Conflict',
    classes: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
  },
  missing: {
    label: 'Missing on PC',
    classes: 'bg-zinc-500/15 text-zinc-700 dark:text-zinc-300',
  },
}

const useVirtual = computed(() => visibleRows.value.length > VIRTUAL_THRESHOLD)
const { list, containerProps, wrapperProps, scrollTo } = useVirtualList(visibleRows, {
  itemHeight: ROW_HEIGHT,
  overscan: 10,
})
const rendered = computed(() =>
  useVirtual.value ? list.value.map((item) => item.data) : visibleRows.value,
)

// Thumbnails are fetched lazily, only for the rows that are actually rendered.
watchEffect(() => {
  for (const row of rendered.value) {
    if (row.photo) void photoStore.loadThumbnail(row.photo.id)
  }
})

watch(filter, () => scrollTo(0))

const allVisibleSelected = computed(
  () => visibleRows.value.length > 0 && visibleRows.value.every((row) => selectedKeys.value.has(row.key)),
)
const someVisibleSelected = computed(() =>
  visibleRows.value.some((row) => selectedKeys.value.has(row.key)),
)

function onToggleAll(event: Event): void {
  comparisonStore.setVisibleSelected((event.target as HTMLInputElement).checked)
}

function subtitle(row: (typeof visibleRows.value)[number]): string {
  if (row.status === 'conflict') {
    return `${row.reason ?? 'content differs'} · ${row.localPath ?? ''}`
  }
  if (row.status === 'missing') return row.localPath ?? ''
  return ''
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div
      class="flex shrink-0 gap-1 overflow-x-auto border-b border-zinc-200 px-4 dark:border-zinc-800"
      role="tablist"
      aria-label="Filter comparison results"
    >
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        role="tab"
        :aria-selected="filter === tab.value"
        class="flex shrink-0 items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        :class="
          filter === tab.value
            ? 'border-blue-500 text-blue-600 dark:text-blue-400'
            : 'border-transparent text-zinc-600 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100'
        "
        @click="comparisonStore.setFilter(tab.value)"
      >
        {{ tab.label }}
        <span
          class="rounded-full bg-zinc-200 px-2 py-0.5 text-xs tabular-nums text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300"
        >
          {{ counts[tab.value] }}
        </span>
      </button>
    </div>

    <div
      v-if="visibleRows.length === 0"
      class="flex flex-1 flex-col items-center justify-center gap-1 p-8 text-center"
    >
      <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">
        {{ counts.all === 0 ? 'Nothing to compare' : 'Nothing in this category' }}
      </p>
      <p class="max-w-sm text-xs text-zinc-500 dark:text-zinc-400">
        {{
          counts.all === 0
            ? 'There are no photos on the iPhone and no images in the folder.'
            : 'Pick another tab to see the other results.'
        }}
      </p>
    </div>

    <div v-else v-bind="containerProps" class="min-h-0 flex-1" aria-label="Comparison results">
      <div
        class="sticky top-0 z-10 grid grid-cols-[32px_48px_minmax(0,1fr)_190px_90px_130px] items-center gap-3 border-b border-zinc-200 bg-zinc-100 px-4 py-2 text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-400"
        role="row"
      >
        <input
          type="checkbox"
          class="h-4 w-4 accent-blue-500"
          aria-label="Select all rows in this category"
          :checked="allVisibleSelected"
          :indeterminate="someVisibleSelected && !allVisibleSelected"
          @change="onToggleAll"
        />
        <span class="sr-only">Preview</span>
        <span>Filename</span>
        <span>Date</span>
        <span class="text-right">Size</span>
        <span>Status</span>
      </div>

      <div v-bind="useVirtual ? wrapperProps : {}">
        <div
          v-for="row in rendered"
          :key="row.key"
          class="grid grid-cols-[32px_48px_minmax(0,1fr)_190px_90px_130px] items-center gap-3 border-b border-zinc-100 px-4 text-sm dark:border-zinc-900"
          :class="selectedKeys.has(row.key) ? 'bg-blue-500/10' : 'hover:bg-zinc-100 dark:hover:bg-zinc-900'"
          :style="{ height: `${ROW_HEIGHT}px` }"
          role="row"
        >
          <input
            type="checkbox"
            class="h-4 w-4 accent-blue-500"
            :aria-label="`Select ${row.filename}`"
            :checked="selectedKeys.has(row.key)"
            @change="comparisonStore.toggleRow(row.key)"
          />

          <div
            class="flex h-12 w-12 items-center justify-center overflow-hidden rounded-md bg-zinc-200 dark:bg-zinc-800"
          >
            <img
              v-if="row.photo && thumbnailCache.get(row.photo.id)"
              :src="thumbnailCache.get(row.photo.id)"
              :alt="row.filename"
              class="h-full w-full object-cover"
              loading="lazy"
            />
            <span
              v-else-if="row.photo && !failedThumbnails.has(row.photo.id)"
              class="h-full w-full animate-pulse bg-zinc-300 dark:bg-zinc-700"
              aria-label="Loading preview"
            />
            <svg
              v-else
              class="h-5 w-5 text-zinc-400 dark:text-zinc-500"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              role="img"
              :aria-label="row.photo ? 'Preview unavailable' : 'File on this PC'"
            >
              <rect x="3" y="5" width="18" height="14" rx="2" />
              <circle cx="9" cy="10" r="1.5" />
              <path d="m21 16-5-5-9 8" />
            </svg>
          </div>

          <div class="min-w-0">
            <p class="truncate font-medium text-zinc-900 dark:text-zinc-100" :title="row.filename">
              {{ row.filename }}
            </p>
            <p
              v-if="subtitle(row)"
              class="truncate text-xs text-zinc-500 dark:text-zinc-400"
              :title="subtitle(row)"
            >
              {{ subtitle(row) }}
            </p>
          </div>

          <span class="truncate tabular-nums text-zinc-600 dark:text-zinc-400">
            {{ formatDateTime(row.date) }}
          </span>
          <span class="text-right tabular-nums text-zinc-600 dark:text-zinc-400">
            {{ formatBytes(row.size) }}
          </span>
          <span
            class="w-fit rounded-full px-2.5 py-0.5 text-xs font-medium"
            :class="badges[row.status].classes"
          >
            {{ badges[row.status].label }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
