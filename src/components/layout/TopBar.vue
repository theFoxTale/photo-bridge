<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDeviceDiscovery } from '../../composables/useDeviceDiscovery'
import { usePhotoStore } from '../../stores/photoStore'
import { translateError } from '../../utils/errors'
import SettingsModal from './SettingsModal.vue'

const { t } = useI18n()
const settingsOpen = ref(false)
const settingsButton = ref<HTMLButtonElement | null>(null)

const { status, activeDevice, error, refresh, stopDiscovery } = useDeviceDiscovery()
const { searchQuery } = storeToRefs(usePhotoStore())

const pill = computed(() => {
  switch (status.value) {
    case 'searching':
      return {
        label: t('status.searching'),
        pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500 animate-pulse',
        animate: 'animate-pulse',
      }
    case 'blocked_by_apple':
      return {
        label: t('status.blocked'),
        pill: 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500',
        animate: '',
      }
    case 'connected':
      return {
        label: activeDevice.value
          ? t('status.connected', { name: activeDevice.value.name })
          : t('status.connectedPlain'),
        pill: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
        dot: 'bg-emerald-500',
        animate: '',
      }
    default:
      return {
        label: t('status.disconnected'),
        pill: 'border-zinc-400/40 bg-zinc-500/10 text-zinc-600 dark:text-zinc-400',
        dot: 'bg-zinc-400 dark:bg-zinc-500',
        animate: '',
      }
  }
})

const pillTitle = computed(() => {
  if (status.value === 'searching') return t('status.clickStop')
  if (status.value === 'blocked_by_apple') return t('status.clickRetryBlocked')
  if (status.value === 'error' && error.value) {
    return t('status.discoveryFailed', { error: translateError(error.value) })
  }
  return t('status.clickSearch')
})

function closeSettings(): void {
  settingsOpen.value = false
  settingsButton.value?.focus()
}

async function onPillClick(): Promise<void> {
  if (status.value === 'searching') {
    await stopDiscovery()
    return
  }
  await refresh()
}
</script>

<template>
  <header
    class="grid h-12 shrink-0 grid-cols-[minmax(0,1fr)_minmax(0,28rem)_minmax(0,1fr)] items-center gap-3 border-b border-zinc-200 bg-white px-4 dark:border-zinc-800 dark:bg-zinc-900"
  >
    <div class="flex min-w-0 items-center gap-2">
      <svg
        class="h-5 w-5 text-blue-500"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <rect x="3" y="5" width="18" height="14" rx="2" />
        <circle cx="9" cy="10" r="1.5" />
        <path d="m21 16-5-5-9 8" />
      </svg>
      <h1 class="truncate text-sm font-semibold tracking-tight text-zinc-900 dark:text-zinc-100">
        {{ $t('app.title') }}
      </h1>
    </div>

    <div class="relative min-w-0">
      <svg
        class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-400"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="11" cy="11" r="7" />
        <path d="m20 20-3.5-3.5" />
      </svg>
      <input
        v-model="searchQuery"
        type="search"
        :placeholder="$t('search.placeholder')"
        autocomplete="off"
        :aria-label="$t('search.aria')"
        class="h-8 w-full rounded-full border border-zinc-300 bg-zinc-50 pl-9 pr-8 text-sm text-zinc-900 outline-none placeholder:text-zinc-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-500/30 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-100 dark:placeholder:text-zinc-500 [&::-webkit-search-cancel-button]:hidden"
      />
      <button
        v-if="searchQuery !== ''"
        type="button"
        class="absolute right-1.5 top-1/2 flex h-6 w-6 -translate-y-1/2 items-center justify-center rounded-full text-zinc-500 hover:bg-zinc-200 hover:text-zinc-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-400 dark:hover:bg-zinc-700 dark:hover:text-zinc-100"
        :aria-label="$t('search.clear')"
        @click="searchQuery = ''"
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
          <path d="M18 6 6 18M6 6l12 12" />
        </svg>
      </button>
    </div>

    <div class="flex items-center justify-end gap-3">
      <button
        type="button"
        class="flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
        :class="pill.pill"
        :title="pillTitle"
        :aria-label="$t('status.aria', { label: pill.label, hint: pillTitle })"
        @click="onPillClick"
      >
        <span class="h-2 w-2 rounded-full" :class="pill.dot" />
        <span :class="pill.animate">{{ pill.label }}</span>
      </button>

      <button
        ref="settingsButton"
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-md text-zinc-600 transition-colors hover:bg-zinc-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-zinc-300 dark:hover:bg-zinc-800"
        :aria-label="$t('settings.open')"
        :title="$t('settings.open')"
        :aria-expanded="settingsOpen"
        @click="settingsOpen = true"
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
          <circle cx="12" cy="12" r="3" />
          <path
            d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"
          />
        </svg>
      </button>
    </div>
  </header>
  <SettingsModal :open="settingsOpen" @close="closeSettings" />
</template>
