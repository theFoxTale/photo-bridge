const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const

/** 1536 -> "1.5 KB". Negative and non-finite values are shown as 0 B. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024
    unit++
  }
  return unit === 0 ? `${Math.round(value)} B` : `${value.toFixed(1)} ${UNITS[unit]}`
}

/** 13002342 -> "12.4 MB/s" */
export function formatSpeed(bytesPerSec: number): string {
  return `${formatBytes(bytesPerSec)}/s`
}

/** 800 -> "<1s", 12400 -> "12s", 65000 -> "1m 05s", 3720000 -> "1h 02m" */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 1000) return '<1s'
  const totalSeconds = Math.round(ms / 1000)
  if (totalSeconds < 60) return `${totalSeconds}s`
  const minutes = Math.floor(totalSeconds / 60)
  if (minutes < 60) return `${minutes}m ${String(totalSeconds % 60).padStart(2, '0')}s`
  return `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, '0')}m`
}
