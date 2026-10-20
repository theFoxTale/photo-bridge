import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'success' | 'warning' | 'error'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
  /** Optional second line with more detail. */
  detail?: string
}

const DEFAULT_DURATION_MS = 8000

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([])

  let nextId = 1
  const timers = new Map<number, ReturnType<typeof setTimeout>>()

  function dismiss(id: number): void {
    const timer = timers.get(id)
    if (timer !== undefined) {
      clearTimeout(timer)
      timers.delete(id)
    }
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }

  /** Shows a toast and removes it after `durationMs`. Returns its id. */
  function show(input: Omit<Toast, 'id'>, durationMs: number = DEFAULT_DURATION_MS): number {
    const id = nextId++
    toasts.value = [...toasts.value, { id, ...input }]
    timers.set(
      id,
      setTimeout(() => dismiss(id), durationMs),
    )
    return id
  }

  return { toasts, show, dismiss }
})
