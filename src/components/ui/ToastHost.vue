<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useToastStore, type ToastKind } from '../../stores/toastStore'

const toastStore = useToastStore()
const { toasts } = storeToRefs(toastStore)

const styles: Record<ToastKind, { box: string; dot: string }> = {
  success: {
    box: 'border-emerald-500/40 bg-emerald-50 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100',
    dot: 'bg-emerald-500',
  },
  warning: {
    box: 'border-amber-500/40 bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-100',
    dot: 'bg-amber-500',
  },
  error: {
    box: 'border-red-500/40 bg-red-50 text-red-900 dark:bg-red-950 dark:text-red-100',
    dot: 'bg-red-500',
  },
}
</script>

<template>
  <div
    class="pointer-events-none fixed bottom-14 right-4 z-50 flex w-96 max-w-[calc(100vw-2rem)] flex-col gap-2"
    role="status"
    aria-live="polite"
  >
    <TransitionGroup
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="translate-y-2 opacity-0"
      leave-active-class="transition duration-150 ease-in"
      leave-to-class="opacity-0"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 text-sm shadow-lg"
        :class="styles[toast.kind].box"
      >
        <span class="mt-1.5 h-2 w-2 shrink-0 rounded-full" :class="styles[toast.kind].dot" />
        <div class="min-w-0 flex-1">
          <p class="font-medium tabular-nums">{{ toast.message }}</p>
          <p v-if="toast.detail" class="mt-1 break-words text-xs opacity-80">{{ toast.detail }}</p>
        </div>
        <button
          type="button"
          class="shrink-0 rounded p-0.5 opacity-60 hover:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          aria-label="Dismiss notification"
          @click="toastStore.dismiss(toast.id)"
        >
          <svg
            class="h-4 w-4"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            aria-hidden="true"
          >
            <path d="M18 6 6 18M6 6l12 12" />
          </svg>
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
