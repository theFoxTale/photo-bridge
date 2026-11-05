<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useSettingsStore, type Language, type Theme } from '../../stores/settingsStore'

const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const settings = useSettingsStore()
const { theme, language } = storeToRefs(settings)

const dialog = ref<HTMLElement | null>(null)

const themes: readonly { value: Theme; labelKey: string }[] = [
  { value: 'light', labelKey: 'settings.light' },
  { value: 'dark', labelKey: 'settings.dark' },
]

const languages: readonly { value: Language; labelKey: string }[] = [
  { value: 'en', labelKey: 'settings.english' },
  { value: 'ru', labelKey: 'settings.russian' },
]

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      window.addEventListener('keydown', onKeydown)
      void nextTick(() => dialog.value?.focus())
    } else {
      window.removeEventListener('keydown', onKeydown)
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
        class="w-full max-w-md rounded-xl border border-zinc-200 bg-white text-zinc-900 shadow-xl outline-none dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100"
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

        <div class="space-y-6 px-5 py-5">
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
