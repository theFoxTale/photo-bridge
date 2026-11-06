<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDeviceDiscovery } from '../../composables/useDeviceDiscovery'
import type { Photo } from '../../types/photo'
import { translateError } from '../../utils/errors'

const props = defineProps<{
  photos: Photo[]
  initialIndex: number
}>()

const emit = defineEmits<{
  close: []
}>()

const { t, locale } = useI18n()
const { activeDeviceId } = useDeviceDiscovery()

const dialog = ref<HTMLElement | null>(null)
const index = ref(props.initialIndex)
const previewUrl = ref<string | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

let request = 0

const current = computed(() => props.photos[index.value] ?? null)
const hasPrev = computed(() => index.value > 0)
const hasNext = computed(() => index.value < props.photos.length - 1)

const dateLabel = computed(() => {
  const raw = current.value?.date ?? ''
  const time = Date.parse(raw)
  if (Number.isNaN(time)) return ''
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(time)
})

function toMessage(err: unknown): string {
  if (typeof err === 'string' && err !== '') return err
  if (err instanceof Error && err.message !== '') return err.message
  return 'Unknown error'
}

function showPrev(): void {
  if (!hasPrev.value) return
  index.value -= 1
}

function showNext(): void {
  if (!hasNext.value) return
  index.value += 1
}

async function loadPreview(photoId: string): Promise<void> {
  const currentRequest = ++request
  loading.value = true
  error.value = null
  previewUrl.value = null

  const deviceId = activeDeviceId.value
  if (deviceId === null) {
    if (currentRequest !== request) return
    loading.value = false
    error.value = 'No device is connected.'
    return
  }

  try {
    const api = window.go?.main?.App
    if (!api?.GetPhotoPreview) {
      throw new Error('Wails runtime is not available. Run the app with `wails dev` or `wails build`.')
    }
    const url = await api.GetPhotoPreview(deviceId, photoId)
    if (currentRequest !== request) return
    if (url === '') {
      throw new Error('Unknown error')
    }
    previewUrl.value = url
  } catch (err) {
    if (currentRequest !== request) return
    error.value = toMessage(err)
  } finally {
    if (currentRequest === request) loading.value = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
    return
  }
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    showPrev()
    return
  }
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    showNext()
  }
}

watch(
  () => current.value?.id,
  (photoId) => {
    if (photoId === undefined) {
      emit('close')
      return
    }
    void loadPreview(photoId)
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  dialog.value?.focus()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  request += 1
})
</script>

<template>
  <div
    ref="dialog"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    class="fixed inset-0 z-[80] bg-black/90 text-white backdrop-blur-sm outline-none"
    :aria-label="current?.filename ?? t('lightbox.loading')"
    @click.self="emit('close')"
  >
    <header class="absolute inset-x-0 top-0 z-10 flex items-start justify-between gap-4 px-4 py-3">
      <div class="min-w-0">
        <p class="truncate text-sm font-medium">{{ current?.filename }}</p>
        <p v-if="dateLabel" class="mt-0.5 text-xs text-white/70">{{ dateLabel }}</p>
      </div>
      <button
        type="button"
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-white/80 hover:bg-white/10 hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
        :aria-label="$t('lightbox.close')"
        @click="emit('close')"
      >
        <svg
          class="h-5 w-5"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M18 6 6 18M6 6l12 12" />
        </svg>
      </button>
    </header>

    <button
      v-if="hasPrev"
      type="button"
      class="absolute left-3 top-1/2 z-10 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-black/40 text-white hover:bg-black/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
      :aria-label="$t('lightbox.previous')"
      @click="showPrev"
    >
      <svg
        class="h-6 w-6"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="m15 18-6-6 6-6" />
      </svg>
    </button>

    <button
      v-if="hasNext"
      type="button"
      class="absolute right-3 top-1/2 z-10 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full bg-black/40 text-white hover:bg-black/60 focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
      :aria-label="$t('lightbox.next')"
      @click="showNext"
    >
      <svg
        class="h-6 w-6"
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
    </button>

    <div class="pointer-events-none flex h-full w-full items-center justify-center px-16">
      <div
        v-if="loading"
        class="pointer-events-auto flex flex-col items-center gap-3 text-sm text-white/80"
        role="status"
      >
        <span
          class="h-8 w-8 animate-spin rounded-full border-2 border-white/20 border-t-white"
          aria-hidden="true"
        />
        <span>{{ $t('lightbox.loading') }}</span>
      </div>
      <div
        v-else-if="error"
        class="pointer-events-auto max-w-md px-6 text-center"
        role="alert"
      >
        <p class="text-sm font-medium">{{ $t('lightbox.failed') }}</p>
        <p class="mt-1 text-xs text-white/70">{{ translateError(error) }}</p>
      </div>
      <img
        v-else-if="previewUrl && current"
        :src="previewUrl"
        :alt="current.filename"
        draggable="false"
        class="pointer-events-auto max-h-[90vh] max-w-[90vw] select-none object-contain"
      />
    </div>
  </div>
</template>
