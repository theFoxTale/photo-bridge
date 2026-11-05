<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSettingsStore, type Language, type Theme } from '../../stores/settingsStore'
import { useToastStore } from '../../stores/toastStore'
import { translateError } from '../../utils/errors'
import { formatBytes } from '../../utils/format'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const settings = useSettingsStore()
const { theme, language } = storeToRefs(settings)
const toastStore = useToastStore()

const dialog = ref<HTMLElement | null>(null)

const themes: readonly { value: Theme; labelKey: string }[] = [
  { value: 'light', labelKey: 'settings.light' },
  { value: 'dark', labelKey: 'settings.dark' },
]

const languages: readonly { value: Language; labelKey: string }[] = [
  { value: 'en', labelKey: 'settings.english' },
  { value: 'ru', labelKey: 'settings.russian' },
]

const cachePath = ref('')
const cacheSize = ref(0)
const cacheLoading = ref(false)
const cacheError = ref<string | null>(null)
const confirmingClear = ref(false)
const clearing = ref(false)
const changing = ref(false)

let cacheRequest = 0

const cacheSizeLabel = computed(() => {
  if (!Number.isFinite(cacheSize.value) || cacheSize.value <= 0) return '0 MB'
  return formatBytes(cacheSize.value)
})

const showCacheLoading = computed(() => cacheLoading.value && cachePath.value === '')
const showCacheLoadError = computed(
  () => cacheError.value !== null && cachePath.value === '' && !cacheLoading.value,
)

function toMessage(err: unknown): string {
  if (typeof err === 'string' && err !== '') return err
  if (err instanceof Error && err.message !== '') return err.message
  return 'Unknown error'
}

function requireApi(): NonNullable<NonNullable<NonNullable<Window['go']>['main']>['App']> {
  const api = window.go?.main?.App
  if (!api) {
    throw new Error('Wails runtime is not available. Run the app with `wails dev` or `wails build`.')
  }
  return api
}

async function loadCacheStats(): Promise<void> {
  const request = ++cacheRequest
  cacheLoading.value = true
  cacheError.value = null
  try {
    const stats = await requireApi().GetCacheStats()
    if (request !== cacheRequest) return
    cachePath.value = typeof stats.path === 'string' ? stats.path : ''
    cacheSize.value = typeof stats.sizeBytes === 'number' ? stats.sizeBytes : 0
  } catch (err) {
    if (request !== cacheRequest) return
    cacheError.value = toMessage(err)
  } finally {
    if (request === cacheRequest) cacheLoading.value = false
  }
}

async function changeCacheFolder(): Promise<void> {
  if (changing.value || clearing.value) return
  changing.value = true
  try {
    const api = requireApi()
    const chosen = await api.SelectCacheFolder()
    if (chosen === '') return
    await api.SetCacheFolder(chosen)
    await loadCacheStats()
    if (cacheError.value) {
      toastStore.show({
        kind: 'error',
        message: t('settings.cache.changeFailed'),
        detail: translateError(cacheError.value),
      })
      return
    }
    toastStore.show({ kind: 'success', message: t('settings.cache.changed') })
  } catch (err) {
    const message = toMessage(err)
    cacheError.value = message
    toastStore.show({
      kind: 'error',
      message: t('settings.cache.changeFailed'),
      detail: translateError(message),
    })
  } finally {
    changing.value = false
  }
}

async function clearCache(): Promise<void> {
  if (clearing.value) return
  clearing.value = true
  try {
    await requireApi().ClearCache()
    confirmingClear.value = false
    await loadCacheStats()
    if (cacheError.value) {
      toastStore.show({
        kind: 'error',
        message: t('settings.cache.clearFailed'),
        detail: translateError(cacheError.value),
      })
      return
    }
    toastStore.show({ kind: 'success', message: t('settings.cache.cleared') })
  } catch (err) {
    const message = toMessage(err)
    cacheError.value = message
    toastStore.show({
      kind: 'error',
      message: t('settings.cache.clearFailed'),
      detail: translateError(message),
    })
  } finally {
    clearing.value = false
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Escape') return
  if (confirmingClear.value && !clearing.value) {
    confirmingClear.value = false
    return
  }
  emit('close')
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      confirmingClear.value = false
      window.addEventListener('keydown', onKeydown)
      void nextTick(() => dialog.value?.focus())
      void loadCacheStats()
    } else {
      window.removeEventListener('keydown', onKeydown)
      confirmingClear.value = false
    }
  },
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})

function onOverlayClick(): void {
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4"
      @click="onOverlayClick"
    >
      <div
        ref="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-title"
        tabindex="-1"
        class="flex max-h-[min(40rem,calc(100vh-2rem))] w-full max-w-md flex-col rounded-xl border border-zinc-200 bg-white text-zinc-900 shadow-xl outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100"
        @click.stop
      >
        <div class="flex items-center justify-between border-b border-zinc-200 px-5 py-4 dark:border-zinc-800">
          <h2 id="settings-title" class="text-base font-semibold">{{ $t('settings.title') }}</h2>
          <button
            type="button"
            class="flex h-8 w-8 items-center justify-center rounded-md text-zinc-500 transition-colors hover:bg-zinc-100 hover:text-zinc-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-400 dark:hover:bg-zinc-800 dark:hover:text-zinc-100"
            :aria-label="$t('settings.close')"
            @click="emit('close')"
          >
            <svg
              class="h-4 w-4"
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
        </div>

        <div class="min-h-0 flex-1 space-y-6 overflow-y-auto px-5 py-5">
          <fieldset>
            <legend class="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
              {{ $t('settings.appearance') }}
            </legend>
            <div class="grid grid-cols-2 gap-2" role="radiogroup" :aria-label="$t('settings.theme')">
              <label
                v-for="option in themes"
                :key="option.value"
                class="flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors focus-within:ring-2 focus-within:ring-blue-500"
                :class="
                  theme === option.value
                    ? 'border-blue-500 bg-blue-500/10 text-blue-700 dark:text-blue-300'
                    : 'border-zinc-300 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800'
                "
              >
                <input
                  type="radio"
                  name="theme"
                  class="h-4 w-4 accent-blue-500"
                  :value="option.value"
                  :checked="theme === option.value"
                  @change="settings.setTheme(option.value)"
                />
                {{ $t(option.labelKey) }}
              </label>
            </div>
          </fieldset>

          <fieldset>
            <legend class="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
              {{ $t('settings.language') }}
            </legend>
            <div class="grid grid-cols-2 gap-2" role="radiogroup" :aria-label="$t('settings.language')">
              <label
                v-for="option in languages"
                :key="option.value"
                class="flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors focus-within:ring-2 focus-within:ring-blue-500"
                :class="
                  language === option.value
                    ? 'border-blue-500 bg-blue-500/10 text-blue-700 dark:text-blue-300'
                    : 'border-zinc-300 hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800'
                "
              >
                <input
                  type="radio"
                  name="language"
                  class="h-4 w-4 accent-blue-500"
                  :value="option.value"
                  :checked="language === option.value"
                  @change="settings.setLanguage(option.value)"
                />
                {{ $t(option.labelKey) }}
              </label>
            </div>
          </fieldset>

          <section class="space-y-3" :aria-label="$t('settings.cache.title')">
            <h3 class="text-xs font-semibold uppercase tracking-wider text-zinc-500 dark:text-zinc-400">
              {{ $t('settings.cache.title') }}
            </h3>

            <div
              v-if="showCacheLoading"
              class="space-y-2"
              aria-busy="true"
              aria-live="polite"
            >
              <div class="h-9 animate-pulse rounded-md bg-zinc-200 dark:bg-zinc-800" />
              <div class="h-4 w-24 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
              <p class="text-xs text-zinc-500 dark:text-zinc-400">{{ $t('settings.cache.loading') }}</p>
            </div>

            <div
              v-else-if="showCacheLoadError"
              class="space-y-2"
              role="alert"
            >
              <p class="text-sm font-medium text-zinc-700 dark:text-zinc-300">{{ $t('settings.cache.loadFailed') }}</p>
              <p class="break-words text-xs text-zinc-500 dark:text-zinc-400">{{ translateError(cacheError) }}</p>
              <button
                type="button"
                class="rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
                @click="loadCacheStats"
              >
                {{ $t('settings.cache.retry') }}
              </button>
            </div>

            <template v-else>
              <div>
                <span class="mb-1.5 block text-xs font-medium text-zinc-700 dark:text-zinc-300">
                  {{ $t('settings.cache.path') }}
                </span>
                <div class="flex items-center gap-2">
                  <p
                    class="min-w-0 flex-1 truncate rounded-md border border-zinc-300 bg-zinc-50 px-3 py-2 text-sm text-zinc-900 dark:border-zinc-700 dark:bg-zinc-950 dark:text-zinc-100"
                    :title="cachePath"
                  >
                    {{ cachePath === '' ? $t('settings.cache.empty') : cachePath }}
                  </p>
                  <button
                    type="button"
                    class="shrink-0 rounded-md border border-zinc-300 px-3 py-2 text-sm font-medium text-zinc-800 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-50 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800"
                    :disabled="changing || clearing || cacheLoading"
                    @click="changeCacheFolder"
                  >
                    {{ changing ? $t('settings.cache.changing') : $t('settings.cache.change') }}
                  </button>
                </div>
              </div>

              <p class="flex items-center justify-between text-sm">
                <span class="text-zinc-600 dark:text-zinc-300">{{ $t('settings.cache.size') }}</span>
                <span class="tabular-nums text-zinc-900 dark:text-zinc-100">{{ cacheSizeLabel }}</span>
              </p>

              <p v-if="cacheError" class="break-words text-xs text-red-500" role="alert">
                {{ translateError(cacheError) }}
              </p>

              <div
                v-if="confirmingClear"
                class="flex items-center justify-between gap-3 rounded-md border border-red-500/40 bg-red-500/10 px-3 py-2"
              >
                <p class="text-sm text-red-700 dark:text-red-300">{{ $t('settings.cache.confirm') }}</p>
                <div class="flex shrink-0 gap-2">
                  <button
                    type="button"
                    class="rounded-md bg-red-600 px-2.5 py-1 text-xs font-medium text-white transition-colors hover:bg-red-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-400 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="clearing"
                    @click="clearCache"
                  >
                    {{ clearing ? $t('settings.cache.clearing') : $t('settings.cache.yes') }}
                  </button>
                  <button
                    type="button"
                    class="rounded-md border border-zinc-300 px-2.5 py-1 text-xs font-medium text-zinc-700 transition-colors hover:bg-white focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-60 dark:border-zinc-600 dark:text-zinc-200 dark:hover:bg-zinc-800"
                    :disabled="clearing"
                    @click="confirmingClear = false"
                  >
                    {{ $t('settings.cache.no') }}
                  </button>
                </div>
              </div>
              <button
                v-else
                type="button"
                class="w-full rounded-md border border-red-500/50 px-3 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-500/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-400 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400"
                :disabled="changing || clearing || cacheLoading"
                @click="confirmingClear = true"
              >
                {{ $t('settings.cache.clear') }}
              </button>
            </template>
          </section>
        </div>

        <div class="flex justify-end border-t border-zinc-200 px-5 py-4 dark:border-zinc-800">
          <button
            type="button"
            class="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-500 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400"
            @click="emit('close')"
          >
            {{ $t('settings.close') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
