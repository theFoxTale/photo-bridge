<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SyncStatus } from '../../types/compare'
import type { Photo } from '../../types/photo'

const props = defineProps<{
  photo: Photo
  selected: boolean
  /** Data URL of the thumbnail, or null while it has not been loaded. */
  thumbUrl: string | null
  thumbFailed: boolean
  syncStatus: SyncStatus
}>()

const emit = defineEmits<{
  /** `only` replaces the selection, `toggle` adds or removes this photo. */
  select: [mode: 'only' | 'toggle']
  /** The tile scrolled into (or near) the viewport; load its thumbnail. */
  visible: []
}>()

const { t } = useI18n()
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

const statusNote = computed(() => {
  switch (props.syncStatus) {
    case 'synced':
      return t('photos.ariaSyncedSuffix')
    case 'conflict':
      return t('photos.ariaConflictSuffix')
    default:
      return ''
  }
})

const ariaLabel = computed(() => {
  const action = props.selected ? t('photos.ariaSelected') : t('photos.ariaSelect')
  const kind = props.photo.isLivePhoto
    ? `${t('photos.kindLive')} `
    : props.photo.isVideo
      ? `${t('photos.kindVideo')} `
      : props.photo.format === 'dng'
        ? `${t('photos.kindRaw')} `
        : ''
  return t('photos.ariaTile', {
    action,
    kind,
    filename: props.photo.filename,
    status: statusNote.value,
  })
})

const checkboxLabel = computed(() =>
  t('photos.checkbox', {
    action: props.selected ? t('photos.ariaDeselect') : t('photos.ariaSelect'),
    filename: props.photo.filename,
  }),
)

const showVideoChrome = computed(() => props.photo.isVideo && (props.thumbUrl !== null || props.thumbFailed))
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
      :aria-label="ariaLabel"
      @click="onTileClick"
    >
      <img
        v-if="thumbUrl"
        :src="thumbUrl"
        :alt="photo.filename"
        draggable="false"
        class="h-full w-full select-none object-cover"
        :class="syncStatus === 'synced' ? 'opacity-50' : ''"
      />
      <span
        v-else-if="thumbFailed && photo.isVideo"
        class="block h-full w-full bg-zinc-900"
        :class="syncStatus === 'synced' ? 'opacity-50' : ''"
      />
      <span
        v-else-if="thumbFailed"
        class="flex h-full w-full flex-col items-center justify-center gap-1 bg-zinc-200 text-xs text-zinc-500 dark:bg-zinc-800 dark:text-zinc-400"
        :class="syncStatus === 'synced' ? 'opacity-50' : ''"
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
        {{ $t('photos.previewUnavailable') }}
      </span>
      <span
        v-else
        class="block h-full w-full animate-pulse bg-zinc-200 dark:bg-zinc-800"
        :class="syncStatus === 'synced' ? 'opacity-50' : ''"
      />

      <span
        v-if="showVideoChrome"
        class="pointer-events-none absolute inset-0"
        :class="syncStatus === 'synced' ? 'opacity-50' : ''"
      >
        <span class="absolute inset-x-0 bottom-0 h-16 bg-linear-to-t from-black/70 to-transparent" />
        <span class="absolute inset-0 flex items-center justify-center">
          <svg class="h-10 w-10 drop-shadow-md" viewBox="0 0 48 48" aria-hidden="true">
            <circle cx="24" cy="24" r="22" fill="black" fill-opacity="0.45" />
            <path fill="white" d="M20 16.5v15l13-7.5-13-7.5z" />
          </svg>
        </span>
      </span>

      <span
        v-if="syncStatus === 'synced'"
        class="pointer-events-none absolute left-2 top-2 flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500 text-white shadow"
        :title="$t('photos.alreadyOnPc')"
      >
        <svg
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
        <span class="sr-only">{{ $t('photos.alreadyOnPc') }}</span>
      </span>
      <span
        v-else-if="syncStatus === 'conflict'"
        class="pointer-events-none absolute left-2 top-2 flex h-5 w-5 items-center justify-center rounded-full bg-amber-500 text-white shadow"
        :title="$t('photos.conflict')"
      >
        <svg
          class="h-3.5 w-3.5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M12 9v4M12 17h.01" />
          <path d="M10.3 4.3 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.3a2 2 0 0 0-3.4 0z" />
        </svg>
        <span class="sr-only">{{ $t('photos.conflict') }}</span>
      </span>

      <span
        v-if="photo.isLivePhoto"
        class="pointer-events-none absolute top-2 flex items-center gap-1 rounded-full bg-black/55 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-white"
        :class="syncStatus === 'synced' || syncStatus === 'conflict' ? 'left-8' : 'left-2'"
        :title="$t('photos.livePhoto')"
      >
        <svg class="h-3 w-3" viewBox="0 0 16 16" aria-hidden="true">
          <circle cx="8" cy="8" r="2.1" fill="currentColor" />
          <circle cx="8" cy="8" r="5.25" fill="none" stroke="currentColor" stroke-width="1.2" />
        </svg>
        LIVE
      </span>

      <span
        v-if="photo.format === 'dng'"
        class="pointer-events-none absolute top-2 rounded bg-black/55 px-1 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-white"
        :class="syncStatus === 'synced' || syncStatus === 'conflict' ? 'left-8' : 'left-2'"
      >
        RAW
      </span>

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
      :aria-label="checkboxLabel"
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
