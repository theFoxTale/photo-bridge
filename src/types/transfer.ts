/** Mirrors the Go `backend.TransferOptions` struct (see backend/transfer.go). */
export interface TransferOptions {
  saveOriginal: boolean
  saveJpg: boolean
  /** 1-100 */
  jpgQuality: number
  destPath: string
  /**
   * Photo ID -> existing file inside `destPath` that the original replaces.
   * Photos not listed keep both files (the new one gets a `_1` suffix).
   */
  overwritePaths?: Record<string, string>
}

/** The part of the options the user edits; the destination is chosen separately. */
export type TransferFormatOptions = Omit<TransferOptions, 'destPath' | 'overwritePaths'>

/** Mirrors the Go `backend.TransferError` struct. */
export interface TransferError {
  photoId: string
  reason: string
}

/** Mirrors the Go `backend.TransferResult` struct. */
export interface TransferResult {
  /** Includes photos that were already in the destination and got skipped. */
  succeeded: string[]
  failed: TransferError[]
  bytesCopied: number
  durationMs: number
  /** True when the transfer was stopped before every photo was processed. */
  cancelled: boolean
}

/** Payload of the `transfer:progress` event (Go `backend.TransferProgress`). */
export interface TransferProgress {
  completed: number
  total: number
  bytesCopied: number
  bytesPerSec: number
  currentFile: string
}
