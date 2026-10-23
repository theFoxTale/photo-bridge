import type { Photo } from './photo'

/** Mirrors the Go `backend.LocalFile` struct (see backend/compare.go). */
export interface LocalFile {
  /** Absolute path */
  path: string
  filename: string
  /** bytes */
  size: number
  /** ISO 8601 */
  modTime: string
}

/** Mirrors the Go `backend.Conflict` struct: same date and size, different content. */
export interface Conflict {
  photo: Photo
  localPath: string
  reason: string
}

/** Mirrors the Go `backend.CompareResult` struct. */
export interface CompareResult {
  newOnDevice: Photo[]
  alreadyOnPc: Photo[]
  conflicts: Conflict[]
  /** On the PC, not on the device. */
  missingOnPc: LocalFile[]
  /** ISO 8601 */
  scannedAt: string
}

export type ComparePhase = 'scanning' | 'listing' | 'hashing' | 'done'

/** Payload of the `compare:progress` event (Go `backend.CompareProgress`). */
export interface CompareProgress {
  scanned: number
  /** 0 while still unknown (scanning and listing). */
  total: number
  phase: ComparePhase
}

/** How a device photo relates to the folder last compared. */
export type SyncStatus = 'synced' | 'new' | 'conflict' | 'unknown'
