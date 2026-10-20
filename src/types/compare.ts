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

export type CompareFilter = 'all' | 'new' | 'existing' | 'conflicts' | 'missing'

/** What to do with photos that conflict with a different local file. */
export type ConflictResolution = 'skip' | 'overwrite' | 'keepBoth'

export type ComparisonStatus = 'new' | 'existing' | 'conflict' | 'missing'

/** One line of the comparison table. */
export interface ComparisonRow {
  /** Unique within a result: `device:<photo id>` or `local:<path>`. */
  key: string
  status: ComparisonStatus
  filename: string
  /** ISO 8601 */
  date: string
  size: number
  /** The device photo; null for files that exist only on the PC. */
  photo: Photo | null
  /** The matching local file (conflicts) or the file itself (missing on PC). */
  localPath: string | null
  /** Why the row is a conflict. */
  reason: string | null
}
